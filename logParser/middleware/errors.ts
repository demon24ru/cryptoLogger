import { NextFunction, Request, Response } from 'express';
import Logger from '../services/logger';
import { NODE_ENV } from '../config';

const logger: Logger = new Logger('Exception');

export const errorMiddleware = (err: any, req: Request, res: Response, next: NextFunction) => {
  logger.error(NODE_ENV !== 'development' ? err.message: err.stack);
  const status = err.status >= 200 && err.status <= 500 ? err.status : 500;
  res.status(status).json({ error: err.message });
}
