import { NestFactory } from '@nestjs/core';
import { Logger } from 'nestjs-pino';
import { DataSource } from 'typeorm';
import { AppModule } from './app.module';

// Top-level await is not available in CJS modules — bootstrap function is required.
async function bootstrap() {
  const app = await NestFactory.create(AppModule, { bufferLogs: true });
  app.useLogger(app.get(Logger));

  const logger = app.get(Logger);

  const dataSource = app.get(DataSource);
  logger.log('Running database migrations', 'Bootstrap');
  const migrations = await dataSource.runMigrations();
  if (migrations.length === 0) {
    logger.log('No pending migrations', 'Bootstrap');
  } else {
    migrations.forEach((m) => logger.log(`Executed migration: ${m.name}`, 'Bootstrap'));
  }

  await app.listen(process.env.PORT ?? 3000);
}

bootstrap(); // NOSONAR: top-level await unavailable in CJS modules
