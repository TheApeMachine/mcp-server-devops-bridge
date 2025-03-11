# Base stage
FROM bitnami/minideb:latest AS base

# Install a comprehensive set of development tools and languages.
# This allows for a versatile development environment capable of
# handling a wide range of programming tasks.
RUN install_packages \
    ca-certificates \
    build-essential \
    python3 \
    python3-pip \
    python3-venv \
    python3-dev \
    python3-setuptools \
    python3-wheel \
    python3-distutils \
    wget \
    curl \
    git \
    jq \
    unzip \
    zip \
    sudo


WORKDIR /root/.ssh
RUN echo "Host github.com\n\tStrictHostKeyChecking no\n" >> /root/.ssh/config

# Create a non-root user with sudo access
ARG USERNAME=agent
ARG USER_UID=1000
ARG USER_GID=$USER_UID

# Create the user
RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME \
    && echo "$USERNAME ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Set up SSH config for the new user
RUN mkdir -p /home/$USERNAME/.ssh \
    && echo "Host github.com\n\tStrictHostKeyChecking no\n" >> /home/$USERNAME/.ssh/config \
    && chown -R $USERNAME:$USERNAME /home/$USERNAME/.ssh

# Switch to the non-root user
USER $USERNAME
WORKDIR /home/$USERNAME
