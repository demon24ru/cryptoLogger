import { NextFunction, Request, Response } from 'express';
import { NotFound } from 'http-errors';
import Logger from '../services/logger';

const logger: Logger = new Logger('Response');

// @ts-ignore
export const defaultResponse = (fn) => (req: Request, res: Response, next: NextFunction) => fn(req, res)
    .then((data: any) => {
        logger.log('');
        res.status(200).json(data);
    })
    .catch((e: any) => next(e));

export const notFoundRouts = (req: Request, res: Response, next: NextFunction) => {
    next(new NotFound('Not found'));
}
