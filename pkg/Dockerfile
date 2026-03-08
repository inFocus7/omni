FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /dash ./app/main.go

FROM gcr.io/distroless/static-debian12

COPY --from=build /dash /dash
COPY ui/templates/ /ui/templates/
COPY ui/static/ /ui/static/

EXPOSE 8080

ENTRYPOINT ["/dash"]
