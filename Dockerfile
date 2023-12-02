FROM golang:1.21
WORKDIR /app
COPY go.mod go.sum main.go ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /goshorty
EXPOSE 8080
CMD ["/goshorty"]