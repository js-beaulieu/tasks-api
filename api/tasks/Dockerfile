FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY api/tasks/go.mod api/tasks/go.sum ./
RUN go mod download

COPY api/tasks/ .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /tasks .

FROM gcr.io/distroless/static
COPY --from=builder /tasks /tasks
EXPOSE 8080
ENTRYPOINT ["/tasks"]
