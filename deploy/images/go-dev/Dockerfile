FROM golang:1.23-bookworm

RUN go install github.com/go-delve/delve/cmd/dlv@v1.23.1

COPY deploy/runner/onespace-supervisor.sh /usr/local/bin/onespace-supervisor
RUN chmod +x /usr/local/bin/onespace-supervisor

WORKDIR /workspace
CMD ["sleep", "infinity"]
