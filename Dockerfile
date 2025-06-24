# Use the official Golang image to create a build artifact.
# This is based on Debian and sets the GOPATH to /go.
FROM golang:1.22 as builder

# Copy local package data to the container's workspace.
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build the binary.
# -o /app/server: Output the binary to /app/server
RUN CGO_ENABLED=0 GOOS=linux go build -v -o server .

# Use a distroless image to run the compiled binary.
FROM gcr.io/distroless/static-debian11

# Copy the binary to the production image from the builder stage.
COPY --from=builder /app/server /app/server

# Expose port 8080 to the outside world
EXPOSE 8080

# Set the entrypoint for the container to run the binary.
ENTRYPOINT ["/app/server"]
