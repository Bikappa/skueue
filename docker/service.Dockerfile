FROM golang:1.19.5-alpine3.17 as builder

COPY service /service-source

WORKDIR /

RUN cd /service-source && go build -o /usr/bin/job-service . && cd / && rm -rf /service-source

EXPOSE 8080
CMD [ "/usr/bin/job-service" ]
