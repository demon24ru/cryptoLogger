import { Request, Response } from 'express';
import Logger from '../services/logger';
import markets from '../store';


const logger: Logger = new Logger('MarketsParser');

// GET
export async function marketsParser(req: Request, res: Response) {
    return markets
}
