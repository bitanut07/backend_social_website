FROM golang:1.25.12-alpine AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build --ldflags "-s -w -extldflags -static" -o main .

FROM alpine:3.22

WORKDIR /www

RUN addgroup -S artly \
    && adduser -S -G artly artly \
    && mkdir -p /www/storage/logs /www/tmp \
    && chown -R artly:artly /www

COPY --from=builder --chown=artly:artly /build/main /www/
COPY --from=builder --chown=artly:artly /build/public/ /www/public/
COPY --from=builder --chown=artly:artly /build/resources/ /www/resources/

USER artly

EXPOSE 3000

ENTRYPOINT ["/www/main"]
