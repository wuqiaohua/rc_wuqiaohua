FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGET=server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${TARGET}

FROM alpine:3.22
WORKDIR /app
COPY --from=build /out/app /app/app
COPY --from=build /src/config /app/config
EXPOSE 8080
ENTRYPOINT ["/app/app"]
