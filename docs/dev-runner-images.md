# Dev Runner Images

Build the Go runner:

```bash
docker build -t onespace/go-dev:1.23 -f deploy/images/go-dev/Dockerfile .
```

Build the Java Maven runner:

```bash
docker build -t onespace/java-dev:21-maven -f deploy/images/java-dev-maven/Dockerfile .
```
