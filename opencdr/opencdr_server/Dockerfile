FROM golang:alpine
RUN apk update && apk add bash 
RUN mkdir -p /app/static

COPY periodic/go.* /app/periodic/
RUN cd /app/periodic && go mod download
COPY /periodic/*.go /app/periodic
RUN cd /app/periodic && go build -o /app/periodic/periodic
RUN echo "* * * * * /app/periodic/periodic" | crontab -


COPY server/go.* /app/server/
RUN cd /app/server && go mod download
COPY /server/*.go /app/server/
RUN cd /app/server && go build -o /app/server/server

COPY start.sh /start.sh
CMD ["/start.sh"]


