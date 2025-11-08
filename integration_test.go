package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"internal-transfers/internal/config"
	"internal-transfers/internal/server"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type IntegrationTestSuite struct {
	suite.Suite
	postgresContainer testcontainers.Container
	serverInstance    *server.Server
	serverPort        string
	baseURL           string
	client            *http.Client
	dbConnStr         string
}

func (suite *IntegrationTestSuite) SetupSuite() {
	ctx := context.Background()

	// Start PostgreSQL container with explicit configuration
	containerReq := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "internal_transfers",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "password",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	postgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: containerReq,
		Started:          true,
	})
	if err != nil {
		suite.T().Fatalf("Failed to start postgres container: %s", err)
	}
	suite.postgresContainer = postgresContainer

	// Get the host and port
	host, err := postgresContainer.Host(ctx)
	if err != nil {
		suite.T().Fatalf("Failed to get container host: %s", err)
	}

	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		suite.T().Fatalf("Failed to get mapped port: %s", err)
	}

	// Build connection string without SSL
	suite.dbConnStr = fmt.Sprintf("host=%s port=%s user=postgres password=password dbname=internal_transfers sslmode=disable",
		host, port.Port())

	// Run migrations
	if err := suite.runMigrations(); err != nil {
		suite.T().Fatalf("Failed to run migrations: %s", err)
	}

	// Start the application server
	if err := suite.startApplicationServer(); err != nil {
		suite.T().Fatalf("Failed to start application server: %s", err)
	}

	suite.client = &http.Client{
		Timeout: 30 * time.Second,
	}
}

func (suite *IntegrationTestSuite) runMigrations() error {
	// Create database connection
	db, err := sql.Open("postgres", suite.dbConnStr)
	if err != nil {
		return err
	}
	defer db.Close()

	// Read migration files from embedded filesystem
	migrationFiles, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migration files by name (version)
	sort.Slice(migrationFiles, func(i, j int) bool {
		return migrationFiles[i].Name() < migrationFiles[j].Name()
	})

	suite.T().Logf("Found %d migration files", len(migrationFiles))

	// Execute migrations in order
	for _, file := range migrationFiles {
		if strings.HasSuffix(file.Name(), ".sql") {
			suite.T().Logf("Executing migration: %s", file.Name())

			migrationPath := filepath.Join("migrations", file.Name())
			migrationSQL, err := migrationsFS.ReadFile(migrationPath)
			if err != nil {
				return fmt.Errorf("failed to read migration file %s: %w", file.Name(), err)
			}

			if _, err := db.Exec(string(migrationSQL)); err != nil {
				return fmt.Errorf("failed to execute migration %s: %w", file.Name(), err)
			}

			suite.T().Logf("Successfully executed migration: %s", file.Name())
		}
	}

	return nil
}

func (suite *IntegrationTestSuite) startApplicationServer() error {
	// Parse connection string components for our config
	cfg := &config.Config{
		DBHost:     "localhost",
		DBPort:     "5432", // This will be overridden by the mapped port
		DBUser:     "postgres",
		DBPassword: "password",
		DBName:     "internal_transfers",
		ServerPort: "0", // Let OS choose a free port
	}

	// Get the actual port from the container
	ctx := context.Background()
	mappedPort, err := suite.postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		return err
	}
	cfg.DBPort = mappedPort.Port()

	// Start server
	serverInstance, port, err := server.StartServer(cfg)
	if err != nil {
		return err
	}

	suite.serverInstance = serverInstance
	suite.serverPort = port
	suite.baseURL = "http://localhost:" + port

	// Wait for server to be ready
	return suite.waitForServerReady()
}

func (suite *IntegrationTestSuite) waitForServerReady() error {
	timeout := 30 * time.Second
	start := time.Now()

	for time.Since(start) < timeout {
		resp, err := http.Get(suite.baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server not ready after %v", timeout)
}

func (suite *IntegrationTestSuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if suite.serverInstance != nil {
		suite.serverInstance.Stop(ctx)
	}

	if suite.postgresContainer != nil {
		suite.postgresContainer.Terminate(ctx)
	}
}

// Helper methods for API calls with better error handling
func (suite *IntegrationTestSuite) createAccount(accountID int64, initialBalance string) (*http.Response, string, error) {
	reqBody := map[string]interface{}{
		"account_id":      accountID,
		"initial_balance": initialBalance,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := suite.client.Post(suite.baseURL+"/accounts", "application/json", bytes.NewReader(body))
	if err != nil {
		return resp, "", err
	}

	// Read response body for debugging
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Create a new response with the body for further processing
	newResp := &http.Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
	}

	return newResp, string(respBody), nil
}

func (suite *IntegrationTestSuite) getAccount(accountID int64) (*http.Response, string, error) {
	resp, err := suite.client.Get(fmt.Sprintf("%s/accounts/%d", suite.baseURL, accountID))
	if err != nil {
		return resp, "", err
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	newResp := &http.Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
	}

	return newResp, string(respBody), nil
}

func (suite *IntegrationTestSuite) transfer(sourceID, destID int64, amount string, idempotencyKey ...string) (*http.Response, string, error) {
	reqBody := map[string]interface{}{
		"source_account_id":      sourceID,
		"destination_account_id": destID,
		"amount":                 amount,
	}

	if len(idempotencyKey) > 0 && idempotencyKey[0] != "" {
		reqBody["idempotency_key"] = idempotencyKey[0]
	}

	body, _ := json.Marshal(reqBody)

	resp, err := suite.client.Post(suite.baseURL+"/transactions", "application/json", bytes.NewReader(body))
	if err != nil {
		return resp, "", err
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	newResp := &http.Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
	}

	return newResp, string(respBody), nil
}

// Helper to parse response and log errors
func (suite *IntegrationTestSuite) parseResponse(body string) (map[string]interface{}, error) {
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		suite.T().Logf("Failed to parse response: %s", body)
		return nil, err
	}
	return response, nil
}

// Helper to compare decimal values properly
func (suite *IntegrationTestSuite) assertDecimalEqual(expected, actual string, msgAndArgs ...interface{}) {
	expectedDec, err := decimal.NewFromString(expected)
	if err != nil {
		suite.T().Fatalf("Invalid expected decimal: %s", expected)
	}

	actualDec, err := decimal.NewFromString(actual)
	if err != nil {
		suite.T().Fatalf("Invalid actual decimal: %s", actual)
	}

	assert.True(suite.T(), expectedDec.Equal(actualDec),
		"Decimal values not equal: expected %s, got %s", expected, actual)
}

// ------------------------------------------------------------------
// Steps below are helpers (non-test methods). They will be executed
// in the order invoked by TestFlow. This allows deterministic ordering
// without relying on test function name prefixes.
// ------------------------------------------------------------------

func (suite *IntegrationTestSuite) stepHealthCheck() {
	resp, err := suite.client.Get(suite.baseURL + "/health")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var healthResp map[string]interface{}
	err = json.Unmarshal(body, &healthResp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "healthy", healthResp["status"])
}

func (suite *IntegrationTestSuite) stepCreateAccounts() {
	// Create first account
	resp, body, err := suite.createAccount(123, "1000.50")
	assert.NoError(suite.T(), err)
	suite.T().Logf("Create Account Response: %s", body)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	// Create second account
	resp, body, err = suite.createAccount(456, "500.25")
	assert.NoError(suite.T(), err)
	suite.T().Logf("Create Account Response: %s", body)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	// Verify accounts were created
	resp, body, err = suite.getAccount(123)
	assert.NoError(suite.T(), err)
	suite.T().Logf("Get Account Response: %s", body)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	// Check the response structure - it should have "data" field
	data, hasData := response["data"]
	assert.True(suite.T(), hasData, "Response should have 'data' field")

	if hasData {
		accountData := data.(map[string]interface{})
		assert.Equal(suite.T(), float64(123), accountData["account_id"])
		// Use decimal comparison instead of string comparison
		suite.assertDecimalEqual("1000.50", accountData["balance"].(string))
	}
}

func (suite *IntegrationTestSuite) stepSuccessfulTransfer() {
	// Perform transfer
	resp, body, err := suite.transfer(123, 456, "200.50")
	assert.NoError(suite.T(), err)
	suite.T().Logf("Transfer Response: %s", body)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	data, hasData := response["data"]
	assert.True(suite.T(), hasData, "Response should have 'data' field")

	if hasData {
		transferData := data.(map[string]interface{})
		assert.Equal(suite.T(), "completed", transferData["status"])
		assert.NotEmpty(suite.T(), transferData["transaction_id"])
	}

	// Verify balances updated
	_, body, err = suite.getAccount(123)
	assert.NoError(suite.T(), err)
	response, err = suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	data, hasData = response["data"]
	if hasData {
		accountData := data.(map[string]interface{})
		// 1000.50 - 200.50 = 800.00
		suite.assertDecimalEqual("800.00", accountData["balance"].(string))
	}

	_, body, err = suite.getAccount(456)
	assert.NoError(suite.T(), err)
	response, err = suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	data, hasData = response["data"]
	if hasData {
		accountData := data.(map[string]interface{})
		// 500.25 + 200.50 = 700.75
		suite.assertDecimalEqual("700.75", accountData["balance"].(string))
	}
}

func (suite *IntegrationTestSuite) stepIdempotentTransfer() {
	idempotencyKey := uuid.New().String()

	// First transfer
	resp, body, err := suite.transfer(123, 456, "100.00", idempotencyKey)
	assert.NoError(suite.T(), err)
	suite.T().Logf("First Transfer Response: %s", body)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	data, hasData := response["data"]
	assert.True(suite.T(), hasData, "Response should have 'data' field")

	var firstTransactionID string
	if hasData {
		transferData := data.(map[string]interface{})
		firstTransactionID = transferData["transaction_id"].(string)
		assert.NotEmpty(suite.T(), firstTransactionID)
	}

	// Second transfer with same idempotency key
	resp, body, err = suite.transfer(123, 456, "100.00", idempotencyKey)
	assert.NoError(suite.T(), err)
	suite.T().Logf("Second Transfer Response: %s", body)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	response, err = suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	data, hasData = response["data"]
	assert.True(suite.T(), hasData, "Response should have 'data' field")

	if hasData {
		transferData := data.(map[string]interface{})
		// Should return the same transaction
		assert.Equal(suite.T(), firstTransactionID, transferData["transaction_id"])
		assert.Equal(suite.T(), "completed", transferData["status"])
	}

	// Verify balance only changed once
	_, body, err = suite.getAccount(123)
	assert.NoError(suite.T(), err)
	response, err = suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	data, hasData = response["data"]
	if hasData {
		accountData := data.(map[string]interface{})
		// 800.00 - 100.00 = 700.00 (only once)
		suite.assertDecimalEqual("700.00", accountData["balance"].(string))
	}
}

func (suite *IntegrationTestSuite) stepNonIdempotentTransfer() {
	// Two transfers without idempotency key should both process
	resp, body, err := suite.transfer(123, 456, "50.00")
	assert.NoError(suite.T(), err)
	suite.T().Logf("First Non-Idempotent Transfer Response: %s", body)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	resp, body, err = suite.transfer(123, 456, "50.00")
	assert.NoError(suite.T(), err)
	suite.T().Logf("Second Non-Idempotent Transfer Response: %s", body)
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	// Verify balance changed twice
	_, body, err = suite.getAccount(123)
	assert.NoError(suite.T(), err)
	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	data, hasData := response["data"]
	if hasData {
		accountData := data.(map[string]interface{})
		// 700.00 - 50.00 - 50.00 = 600.00
		suite.assertDecimalEqual("600.00", accountData["balance"].(string))
	}
}

func (suite *IntegrationTestSuite) stepInsufficientBalance() {
	// Try to transfer more than available balance
	resp, body, err := suite.transfer(123, 456, "10000.00")
	assert.NoError(suite.T(), err)
	suite.T().Logf("Insufficient Balance Response: %s", body)
	assert.Equal(suite.T(), http.StatusUnprocessableEntity, resp.StatusCode)

	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	errorData, hasError := response["error"]
	assert.True(suite.T(), hasError, "Response should have 'error' field for error cases")

	if hasError {
		errorInfo := errorData.(map[string]interface{})
		assert.Equal(suite.T(), "insufficient_balance", errorInfo["code"])
	}

	// Verify balances unchanged
	_, body, err = suite.getAccount(123)
	assert.NoError(suite.T(), err)
	response, err = suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	data, hasData := response["data"]
	if hasData {
		accountData := data.(map[string]interface{})
		// Should remain 600.00 (unchanged)
		suite.assertDecimalEqual("600.00", accountData["balance"].(string))
	}
}

func (suite *IntegrationTestSuite) stepSameAccountTransfer() {
	// Try to transfer to same account
	resp, body, err := suite.transfer(123, 123, "100.00")
	assert.NoError(suite.T(), err)
	suite.T().Logf("Same Account Transfer Response: %s", body)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)

	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	errorData, hasError := response["error"]
	assert.True(suite.T(), hasError, "Response should have 'error' field for error cases")

	if hasError {
		errorInfo := errorData.(map[string]interface{})
		assert.Equal(suite.T(), "same_account_transfer", errorInfo["code"])
	}
}

func (suite *IntegrationTestSuite) stepInvalidAmount() {
	// Try to transfer negative amount
	resp, body, err := suite.transfer(123, 456, "-100.00")
	assert.NoError(suite.T(), err)
	suite.T().Logf("Invalid Amount Response: %s", body)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)

	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	errorData, hasError := response["error"]
	assert.True(suite.T(), hasError, "Response should have 'error' field for error cases")

	if hasError {
		errorInfo := errorData.(map[string]interface{})
		assert.Equal(suite.T(), "invalid_amount", errorInfo["code"])
	}
}

func (suite *IntegrationTestSuite) stepZeroAmount() {
	// Try to transfer zero amount
	resp, body, err := suite.transfer(123, 456, "0.00")
	assert.NoError(suite.T(), err)
	suite.T().Logf("Zero Amount Response: %s", body)
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)

	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	errorData, hasError := response["error"]
	assert.True(suite.T(), hasError, "Response should have 'error' field for error cases")

	if hasError {
		errorInfo := errorData.(map[string]interface{})
		assert.Equal(suite.T(), "invalid_amount", errorInfo["code"])
	}
}

func (suite *IntegrationTestSuite) stepAccountNotFound() {
	// Try to get non-existent account
	resp, body, err := suite.getAccount(999)
	assert.NoError(suite.T(), err)
	suite.T().Logf("Account Not Found Response: %s", body)
	assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)

	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	errorData, hasError := response["error"]
	assert.True(suite.T(), hasError, "Response should have 'error' field for error cases")

	if hasError {
		errorInfo := errorData.(map[string]interface{})
		assert.Equal(suite.T(), "account_not_found", errorInfo["code"])
	}
}

func (suite *IntegrationTestSuite) stepDuplicateAccountCreation() {
	// Try to create account with same ID
	resp, body, err := suite.createAccount(123, "500.00")
	assert.NoError(suite.T(), err)
	suite.T().Logf("Duplicate Account Response: %s", body)
	assert.Equal(suite.T(), http.StatusConflict, resp.StatusCode)

	response, err := suite.parseResponse(body)
	assert.NoError(suite.T(), err)

	errorData, hasError := response["error"]
	assert.True(suite.T(), hasError, "Response should have 'error' field for error cases")

	if hasError {
		errorInfo := errorData.(map[string]interface{})
		assert.Equal(suite.T(), "duplicate_account", errorInfo["code"])
	}
}

func (suite *IntegrationTestSuite) TestFlow() {
	if testing.Short() {
		suite.T().Skip("Skipping integration test in short mode")
	}

	suite.stepHealthCheck()
	suite.stepCreateAccounts()
	suite.stepSuccessfulTransfer()
	suite.stepIdempotentTransfer()
	suite.stepNonIdempotentTransfer()
	suite.stepInsufficientBalance()
	suite.stepSameAccountTransfer()
	suite.stepInvalidAmount()
	suite.stepZeroAmount()
	suite.stepAccountNotFound()
	suite.stepDuplicateAccountCreation()
}

func TestIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(IntegrationTestSuite))
}
