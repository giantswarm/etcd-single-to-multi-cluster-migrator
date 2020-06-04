FROM alpine:3.11

RUN apk add --update ca-certificates \
    && rm -rf /var/cache/apk/*

ADD ./etcd-cluster-migrator /etcd-cluster-migrator

ENTRYPOINT ["/etcd-cluster-migrator"]
