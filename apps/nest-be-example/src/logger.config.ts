import { trace } from '@opentelemetry/api';
import { Params } from 'nestjs-pino';

export const loggerConfig: Params = {
  pinoHttp: {
    level: process.env.LOG_LEVEL ?? 'info',
    mixin() {
      const span = trace.getActiveSpan();
      if (!span?.isRecording()) return {};
      const { traceId, spanId } = span.spanContext();
      return { traceId, spanId };
    },
    transport:
      process.env.NODE_ENV !== 'production'
        ? { target: 'pino-pretty', options: { singleLine: true } }
        : undefined,
  },
};
