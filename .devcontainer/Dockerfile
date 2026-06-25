FROM ghcr.io/lesomnus/dev-golang:1.25

RUN TEMP=$(mktemp -d); \
	ARCH=$(uname -m); \
	VERSION="31.1"; \
	case "$ARCH" in \
		'x86_64') ;; \
		'aarch64') \
			ARCH='aarch_64'; \
			;; \
		*) exit 1 ;; \
	esac; \
	cd "${TEMP}" \
	&& curl -o ./protoc.zip -fsSL "https://github.com/protocolbuffers/protobuf/releases/download/v${VERSION}/protoc-${VERSION}-linux-$ARCH.zip" \
	&& unzip ./protoc.zip -d /usr/local \
	&& chmod 755 /usr/local/bin/protoc \
	&& rm -rf "${TEMP}"

RUN VERSION="1.55.1"; \
	curl -fsSL "https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf-$(uname -s)-$(uname -m).tar.gz" \
		| tar -xvzf - -C /usr/local --strip-components 1 buf/etc buf/bin buf/share/zsh

RUN --mount=type=cache,target=/home/hypnos/.cache/go-build \
	--mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/go/pkg/sumdb \
	echo \
		google.golang.org/protobuf/cmd/protoc-gen-go@v1.36 \
		google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5 \
	| xargs -n1 go install
