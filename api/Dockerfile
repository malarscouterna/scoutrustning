# --- Dev stage: hot reload with air ---
FROM golang:1.26-alpine AS dev
RUN apk add --no-cache gcc musl-dev vips-dev
RUN go install github.com/air-verse/air@latest
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
ENV MIGRATIONS_DIR=/src/migrations
EXPOSE 8080
CMD ["air"]

# --- Production build ---
FROM golang:1.26-alpine AS build
RUN apk add --no-cache gcc musl-dev vips-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /bin/server ./cmd/server

FROM alpine:3.22 AS production
RUN apk add --no-cache ca-certificates tzdata vips
COPY --from=build /bin/server /bin/server
COPY --from=build /src/migrations /migrations
ENV MIGRATIONS_DIR=/migrations
EXPOSE 8080
ENTRYPOINT ["/bin/server"]
