FROM golang:1.26-alpine
 
WORKDIR /

COPY go.mod go.sum ./
 
RUN go mod download

COPY cmd cmd
COPY config.json config.json

RUN CGO_ENABLED=0 go build -o ./main ./cmd/web
 
EXPOSE 8080

CMD [ "./main" ]