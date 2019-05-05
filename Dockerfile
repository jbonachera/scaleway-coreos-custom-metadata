FROM vxlabs/glide as builder

WORKDIR $GOPATH/src/github.com/jbonachera/scaleway-coreos-custom-metadata
COPY glide* ./
RUN glide install -v
COPY . ./
RUN go test $(glide nv) && \
    go build -ldflags="-s -w" -buildmode=exe -a -o /bin/scaleway-coreos-custom-metadata ./main.go

FROM alpine
COPY --from=builder /bin/scaleway-coreos-custom-metadata /bin/scaleway-coreos-custom-metadata

