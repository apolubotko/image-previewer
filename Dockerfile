ARG PROJECT=image-previewer

# Step 1
FROM golang:1.16.4-buster as gobuilder
ARG PROJECT
WORKDIR /go/src/${PROJECT} 
COPY . .
RUN go build -v -o /usr/local/sbin/${PROJECT} ./cmd/${PROJECT}/main.go 

# # Step 2

FROM ubuntu:22.04
ARG PROJECT
ENV CMD_PROJ=${PROJECT}
COPY --from=gobuilder /usr/local/sbin/${PROJECT} /${PROJECT}
CMD ["sh","-c","/${CMD_PROJ}"]