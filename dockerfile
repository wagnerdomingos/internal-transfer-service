FROM golang:1.25-alpine

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o main ./cmd/server

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]