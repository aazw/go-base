ARG GOLANG_VERSION=1.24

# ##############################
# builder images
# ##############################

FROM golang:${GOLANG_VERSION} AS go-builder

# COPY go.mod go.sum main.go /goapp/
COPY . /goapp/
WORKDIR /goapp/
ENV GOOS=linux
RUN go build -o /goapp/goapp main.go

# ##############################
# main image
# ##############################

FROM debian
COPY --from=go-builder /goapp/goapp /usr/local/bin/

EXPOSE 8080
CMD ["goapp"]