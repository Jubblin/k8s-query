FROM golang:1.21-bullseye

# Install basic development tools
RUN apt-get update && apt-get install -y \
  curl \
  git \
  && rm -rf /var/lib/apt/lists/*

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
  && chmod +x kubectl \
  && mv kubectl /usr/local/bin/

# Create non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME \
  && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME \
  && apt-get update \
  && apt-get install -y sudo direnv \
  && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
  && chmod 0440 /etc/sudoers.d/$USERNAME

# Set up Go workspace
ENV GOPATH /home/$USERNAME/go
ENV PATH $GOPATH/bin:$PATH

# Create Go workspace directory
RUN mkdir -p $GOPATH \
  && chown -R $USERNAME:$USERNAME $GOPATH

# Switch to non-root user
USER $USERNAME

# Set working directory
WORKDIR /workspace

# Create .kube directory
RUN go install github.com/go-delve/delve/cmd/dlv@latest && \
    mkdir -p /home/$USERNAME/.kube

# Set default shell to bash
SHELL ["/bin/bash", "-c"] 