# cd gofer => ls: cmd go.mod go.sum gofer static statik ...
openssl req -new -nodes -x509 -out static/certs/server.pem -keyout static/certs/server.key -days 3650 -subj "/C=DE/ST=NRW/L=Earth/O=Gofer/OU=IT/CN=gofer.cdfmlr.mine/emailAddress=gofer@mine.io"
openssl req -new -nodes -x509 -out static/certs/client.pem -keyout static/certs/client.key -days 3650 -subj "/C=DE/ST=NRW/L=Earth/O=Gofer/OU=IT/CN=gofer.cdfmlr.mine/emailAddress=gofer@mine.io"
statik -src=static
