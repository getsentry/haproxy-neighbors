FROM golang:1.15-rc-alpine AS builder

RUN apk add --no-cache make

RUN mkdir -p /usr/src
WORKDIR /usr/src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN make

FROM haproxy:2.2.0-alpine
COPY --from=builder /usr/src/haproxy-neighbors /usr/local/bin/

CMD ["/usr/local/bin/haproxy-neighbors"]
