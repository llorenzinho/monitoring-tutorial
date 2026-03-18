import { MigrationInterface, QueryRunner } from 'typeorm';

export class CreateItemTable1710000000000 implements MigrationInterface {
  name = 'CreateItemTable1710000000000';

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`
      CREATE TABLE IF NOT EXISTS "item" (
        "id"          SERIAL                    NOT NULL,
        "name"        character varying         NOT NULL,
        "description" character varying,
        "createdAt"   TIMESTAMP WITH TIME ZONE  NOT NULL DEFAULT now(),
        CONSTRAINT "PK_item" PRIMARY KEY ("id")
      )
    `);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`DROP TABLE "item"`);
  }
}
