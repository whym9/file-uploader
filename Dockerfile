FROM golang:latest as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN GOOS=linux go build upload-page .

FROM alpine:latest
COPY --from=builder /build/ .
ENTRYPOINT ["./main"]
CMD [ "localhost:8080","./files",2*1024*1024 ]

