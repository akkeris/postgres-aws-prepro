FROM golang:1.8-alpine
RUN echo "extra"
RUN apk update
RUN apk add git
RUN apk add tzdata
RUN cp /usr/share/zoneinfo/America/Denver /etc/localtime
ADD root /var/spool/cron/crontabs/root
RUN mkdir -p /go/src/oct-postgres-preprovision
ADD oct-postgres-preprovision.go  /go/src/oct-postgres-preprovision/oct-postgres-preprovision.go
ADD create.sql /go/src/oct-postgres-preprovision/create.sql
ADD build.sh /build.sh
RUN chmod +x /build.sh
RUN /build.sh
#CMD ["/go/src/oct-postgres-preprovision/oct-postgres-preprovision"]
CMD ["crond", "-f"]
#RUN mkdir /root/.aws
#ADD credentials /root/.aws/credentials




