FROM ubuntu

COPY ./alertmanager-imessage-tunnel /app

ENTRYPOINT [ "/app/alertmanager-imessage-tunnel" ]
