FROM golang:1.20-alpine3.17 AS builder

WORKDIR /app

RUN apk add --update --no-cache curl git

RUN curl --fail -s -o /bin/healthcheck https://artifacts-artifactsbucket-2ce0jkp2lskq.s3.eu-central-1.amazonaws.com/crg/ts-deploy/bin/healthcheck-arm64-alpine && \
	chmod a+x /bin/healthcheck

COPY go.mod ./
COPY go.sum ./

# ssh setup for CRG go packages
RUN apk add openssh-client
RUN mkdir /root/.ssh
COPY known_hosts /root/.ssh/known_hosts
COPY packages_id_rsa /root/.ssh/id_rsa
RUN chmod 600 /root/.ssh/id_rsa
RUN eval $(ssh-agent -s) && ssh-add /root/.ssh/id_rsa
RUN git config --global url.git@bitbucket.org:.insteadOf https://bitbucket.org/
ENV GOPRIVATE=bitbucket.org

RUN go mod download

COPY . ./

RUN go build -o /build/main ./cmd/server.go

LABEL com.carrentalgateway.build.intermediate-tag=services/supplier-hub/intermediate/build

# Final image
FROM alpine:3.17

COPY --from=builder /bin/healthcheck /bin

WORKDIR /app
COPY --from=builder /build/main /usr/local/bin/main
COPY --from=builder /app/api/openapi.json /app/openapi.json

ENV OPENAPI_LOCATION=/app/openapi.json
CMD ["main"]
