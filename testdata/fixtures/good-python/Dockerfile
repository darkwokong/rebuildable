FROM python:3.12.1-slim AS rebuildable-test

WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
CMD ["python", "-m", "pytest"]

FROM python:3.12.1-slim AS runtime

WORKDIR /app
COPY . .
CMD ["python", "app.py"]
