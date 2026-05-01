FROM maven:3.9-eclipse-temurin-21

COPY deploy/runner/onespace-supervisor.sh /usr/local/bin/onespace-supervisor
RUN chmod +x /usr/local/bin/onespace-supervisor

WORKDIR /workspace
CMD ["sleep", "infinity"]
