# infrastructure-bundle is not multiarch yet, so we use as a base

# TODO TODO TODO TODO
# TODO: CHANGE THIS
# TODO TODO TODO TODO
ARG IMAGE_NAME=carlosroman/nri-test
ARG IMAGE_TAG=latest
ARG MODE=normal

FROM $IMAGE_NAME:$IMAGE_TAG AS base

# Set by docker buildx automatically
ARG TARGETOS
ARG TARGETARCH

# ensure there is no default integration enabled
RUN rm -rf /etc/newrelic-infra/integrations.d/*
ADD nri-kubernetes-definition.yml /var/db/newrelic-infra/newrelic-integrations/
ADD bin/nri-kubernetes-${TARGETOS}-${TARGETARCH} /var/db/newrelic-infra/newrelic-integrations/bin/nri-kubernetes

# Warning: First, Edit sample file to suit your needs and rename it to
# `nri-kubernetes-config.yml`
ADD nri-kubernetes-config.yml.sample /var/db/newrelic-infra/integrations.d/nri-kubernetes-config.yml

FROM base AS branch-normal
USER root

FROM base AS branch-unprivileged
RUN addgroup -g 2000 nri-agent && adduser -D -u 1000 -G nri-agent nri-agent
USER nri-agent

ENV NRIA_OVERRIDE_HOST_ROOT ""
ENV NRIA_IS_SECURE_FORWARD_ONLY true

FROM branch-${MODE}
ENTRYPOINT ["/usr/bin/newrelic-infra"]
