FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /quote-engine .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /quote-engine /quote-engine
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/quote-engine"]
