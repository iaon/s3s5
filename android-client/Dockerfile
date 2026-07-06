FROM ghcr.io/cirruslabs/android-sdk:36

ARG GRADLE_VERSION=9.4.1

USER root

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl unzip \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip" -o /tmp/gradle.zip \
    && unzip -q /tmp/gradle.zip -d /opt \
    && ln -sf "/opt/gradle-${GRADLE_VERSION}/bin/gradle" /usr/local/bin/gradle \
    && rm /tmp/gradle.zip

RUN yes | sdkmanager --licenses >/dev/null || true \
    && sdkmanager "platforms;android-36" "build-tools;36.0.0" "build-tools;35.0.0" "platform-tools"

WORKDIR /work/android-client
