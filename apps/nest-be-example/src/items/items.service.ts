import { Injectable, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { PinoLogger, InjectPinoLogger } from 'nestjs-pino';
import { Repository } from 'typeorm';
import { Item } from './item.entity';

@Injectable()
export class ItemsService {
  constructor(
    @InjectRepository(Item)
    private readonly itemsRepository: Repository<Item>,
    @InjectPinoLogger(ItemsService.name)
    private readonly logger: PinoLogger,
  ) {}

  findAll(): Promise<Item[]> {
    this.logger.info('Fetching all items');
    return this.itemsRepository.find();
  }

  async findOne(id: number): Promise<Item> {
    this.logger.info({ id }, 'Fetching item');
    const item = await this.itemsRepository.findOneBy({ id });
    if (!item) {
      this.logger.warn({ id }, 'Item not found');
      throw new NotFoundException(`Item #${id} not found`);
    }
    return item;
  }

  create(data: { name: string; description?: string }): Promise<Item> {
    this.logger.info({ name: data.name }, 'Creating item');
    const item = this.itemsRepository.create(data);
    return this.itemsRepository.save(item);
  }

  async remove(id: number): Promise<void> {
    this.logger.info({ id }, 'Removing item');
    const item = await this.findOne(id);
    await this.itemsRepository.remove(item);
  }
}
