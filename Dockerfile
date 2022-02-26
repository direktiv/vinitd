FROM golang:1.14.15-buster

WORKDIR /vinitd
COPY . /vinitd

COPY run.sh /run.sh
RUN chmod 755 /run.sh

CMD ["/run.sh"]