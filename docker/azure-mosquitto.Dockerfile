FROM eclipse-mosquitto:2.0

# Create mosquitto configuration directory if it doesn't exist
RUN mkdir -p /mosquitto/config

# Create mosquitto configuration file for Azure (allows remote connections)
RUN echo 'listener 1883 0.0.0.0' > /mosquitto/config/mosquitto.conf && \
    echo 'allow_anonymous true' >> /mosquitto/config/mosquitto.conf && \
    echo '' >> /mosquitto/config/mosquitto.conf && \
    echo 'listener 8883 0.0.0.0' >> /mosquitto/config/mosquitto.conf && \
    echo 'protocol websockets' >> /mosquitto/config/mosquitto.conf && \
    echo 'allow_anonymous true' >> /mosquitto/config/mosquitto.conf && \
    echo '' >> /mosquitto/config/mosquitto.conf && \
    echo 'log_dest stdout' >> /mosquitto/config/mosquitto.conf && \
    echo 'log_type all' >> /mosquitto/config/mosquitto.conf && \
    echo 'connection_messages true' >> /mosquitto/config/mosquitto.conf

# Use the default mosquitto entrypoint
# The configuration file is already in place, so mosquitto will use it automatically

