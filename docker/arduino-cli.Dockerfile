FROM golang:1.19.5-alpine3.17 as builder

RUN apk add curl gcompat libc6-compat && \
    curl -fsSL https://raw.githubusercontent.com/arduino/arduino-cli/master/install.sh | sh

WORKDIR /

CMD [ "arduino-cli" ]