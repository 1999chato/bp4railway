FROM alpine:3.14
ARG PORT
ARG WEB_HOST
ARG WEB_USERNAME
ARG WEB_PASSWORD
WORKDIR /
ENV NPS_RELEASE_URL https://github.com/ehang-io/nps/releases/download/v0.26.10/linux_amd64_server.tar.gz

RUN set -x && \
	wget --no-check-certificate ${NPS_RELEASE_URL} && \ 
	tar xzf linux_amd64_server.tar.gz && \
	rm linux_amd64_server.tar.gz

RUN echo -e 'bridge_type=tcp\n\
bridge_port=${PORT}\n\
bridge_ip=0.0.0.0\n\
web_username=${WEB_USERNAME}\n\
web_password=${WEB_PASSWORD}\n\
web_port=${PORT}\n\
web_ip=0.0.0.0\n\
web_host=${WEB_HOST}\n'\
> /conf/nps.conf

CMD /nps