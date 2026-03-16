FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go install github.com/evanw/esbuild/cmd/esbuild@latest && \
    esbuild ui/static/app.js --minify --outfile=ui/static/app.min.js

# Build the full app package (picks up main.go + k8s.go together)
RUN CGO_ENABLED=0 go build -o /omni ./app

FROM gcr.io/distroless/static-debian12

COPY --from=build /omni /omni
COPY ui/templates/ /ui/templates/
COPY ui/static/ /ui/static/

EXPOSE 8080

ENTRYPOINT ["/omni"]
