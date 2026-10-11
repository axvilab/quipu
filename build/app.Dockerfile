# Build image for quipu-app (the Wails window, ADR 0007): Go plus the GTK 3 and WebKitGTK 4.1
# headers that Wails links against on Linux.
ARG GO_IMAGE=golang:1.27.2
FROM ${GO_IMAGE}
RUN apt-get update \
	&& apt-get install -y --no-install-recommends libgtk-3-dev libwebkit2gtk-4.1-dev \
	&& rm -rf /var/lib/apt/lists/*
