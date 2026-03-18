import 'reflect-metadata';
import { DataSource } from 'typeorm';
import { Item } from '../items/item.entity';

export const AppDataSource = new DataSource({
  type: 'postgres',
  host: process.env.DB_HOST ?? 'localhost',
  port: Number.parseInt(process.env.DB_PORT ?? '5432', 10),
  username: process.env.DB_USER ?? 'postgres',
  password: process.env.DB_PASSWORD ?? 'postgres',
  database: process.env.DB_NAME ?? 'nest_example',
  entities: [Item],
  migrations: ['src/migrations/*.ts'],
});
