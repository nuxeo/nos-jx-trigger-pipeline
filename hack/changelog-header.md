### Linux

```shell
curl -L https://github.com/jenkins-x-labs/trigger-pipeline/releases/download/v{{.Version}}/tp-linux-amd64.tar.gz | tar xzv 
sudo mv tp /usr/local/bin
```

### macOS

```shell
curl -L  https://github.com/jenkins-x-labs/trigger-pipeline/releases/download/v{{.Version}}/tp-darwin-amd64.tar.gz | tar xzv
sudo mv tp /usr/local/bin
```

