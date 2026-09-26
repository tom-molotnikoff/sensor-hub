FROM python:3.14-alpine

WORKDIR /app

COPY mocks/requirements.txt .
RUN --mount=type=cache,target=/root/.cache/pip pip install -r requirements.txt

COPY mocks/ .
