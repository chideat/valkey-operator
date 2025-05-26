FROM build-harbor.alauda.cn/acp/violet:v3.0.0 as violet

FROM build-harbor.alauda.cn/ops/ubuntu:22.04
WORKDIR /app


COPY --from=violet /bin/violet_linux_amd64 violet_linux_amd64
COPY --from=violet /bin/violet_linux_arm64 violet_linux_arm64

RUN set -ex; \
    sed -i 's@/archive.ubuntu.com/@/mirrors.tuna.tsinghua.edu.cn/@g' /etc/apt/sources.list; \
    sed -i 's@/security.ubuntu.com/@/mirrors.tuna.tsinghua.edu.cn/@g' /etc/apt/sources.list; \
    apt clean; \
    apt update; \
    apt install -y curl

RUN set -ex; \
    arch=$(arch); \
    case "$arch" in \
        x86_64) \
            std_arch="amd64" \
            ;; \
        aarch64) \
            std_arch="arm64" \
            ;; \
        *) \
            std_arch="$arch" \
            ;;\
    esac; \
    cp violet_linux_${std_arch} /usr/local/bin/violet; \
    curl https://dl.min.io/client/mc/release/linux-${std_arch}/mc -o /usr/local/bin/mc && chmod +x /usr/local/bin/mc;