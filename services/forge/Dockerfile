FROM python:3.12-slim AS base

WORKDIR /app

RUN pip install --no-cache-dir \
    numpy \
    pandas \
    scipy \
    numba

COPY . .

EXPOSE 8500 8501 8502

ENTRYPOINT ["python", "-m", "forge"]
