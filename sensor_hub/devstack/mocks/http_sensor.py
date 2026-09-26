import os
import random
import datetime
from flask import Flask, jsonify


def _init_telemetry():
    """Initialise OpenTelemetry tracing if OTEL_EXPORTER_OTLP_ENDPOINT is set."""
    endpoint = os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT")
    if not endpoint:
        return
    try:
        from opentelemetry import trace
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter

        resource = Resource.create({
            "service.name": os.environ.get("OTEL_SERVICE_NAME", "mock-sensor")
        })
        provider = TracerProvider(resource=resource)
        exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)
        provider.add_span_processor(BatchSpanProcessor(exporter))
        trace.set_tracer_provider(provider)
    except ImportError:
        pass


_init_telemetry()

app = Flask(__name__)

try:
    from opentelemetry.instrumentation.flask import FlaskInstrumentor
    FlaskInstrumentor().instrument_app(app)
except ImportError:
    pass


@app.route('/temperature')
def temperature():
    temp = round(random.uniform(18.0, 22.0), 2)
    now = datetime.datetime.now(datetime.timezone.utc)
    formatted = now.strftime("%Y-%m-%d %H:%M:%S")
    return jsonify({"temperature": temp, "time": formatted})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)