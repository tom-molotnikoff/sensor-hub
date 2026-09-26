FROM python:3.14-alpine

WORKDIR /app

COPY mocks/requirements.txt .
RUN --mount=type=cache,id=sensor-hub-dev-pip,target=/root/.cache/pip pip install -r requirements.txt

COPY mocks/ .
