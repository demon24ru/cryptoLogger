import { Request, Response } from 'express';
import fs from 'fs/promises';
import Logger from '../services/logger';
import {InternalServerError} from 'http-errors';
import {LOG_FILE_MASK_NAME} from '../config';


const logger: Logger = new Logger('LogParser');

const minWorkTime: number = 5*60*1000 // 5 min
const timeBeforeStart: number = 10*60*1000 // 10 min

// tslint:disable-next-line:class-name
interface logData {
    time: string
    exchange?: string
    message?: string
}

// GET
export async function logParser(req: Request, res: Response) {
    const {file, exchange, market} = req.query
    if (!file || !exchange || !market) {
        throw new InternalServerError()
    }

    let marketName: string = market as string
    switch (exchange) {
        case 'kucoin':
            // @ts-ignore
            if (market.slice(-1) === 'F') {
                // @ts-ignore
                marketName = market.slice(0, -1) + 'M'
            } else {
                // @ts-ignore
                marketName = market.slice(0, -4) + '-USDT'
            }
            break
        case 'bybit':
            // @ts-ignore
            if (market.slice(-1) === 'F') {
                // @ts-ignore
                marketName = market.slice(0, -1)
            }
            break
        case 'binance':
            // @ts-ignore
            if (market.slice(-1) === 'F') {
                // @ts-ignore
                marketName = market.slice(0, -1)
            }
            break
    }
    let exchangeName: string = exchange as string
    // @ts-ignore
    if (market.slice(-1) === 'F') {
        // @ts-ignore
        exchangeName = exchange + 'Futures'
    }

    try {
        const data = await fs.readFile(LOG_FILE_MASK_NAME + file + '.log', { encoding: 'utf8' });
        const lines: string[] = data.split('\n').slice(0, -1)
        const response = []
        let timeStart = 0
        // tslint:disable-next-line:forin
        for (const k in lines) {
            const dat: logData = JSON.parse(lines[k])
            if (!dat.message) {
                continue
            }
            const time: number = new Date(dat.time).getTime()
            if (dat.message === 'exiting the app') {
                if (response[response.length-1].length === 1) {
                    if ((time - timeStart) < minWorkTime) {
                        response.pop()
                    } else {
                        response[response.length-1].push(time)
                    }
                }
                continue
            }
            // @ts-ignore
            if (k > 0 && dat.message === 'start app') {
                if (response[response.length-1].length === 1) {
                    if ((time - timeBeforeStart - timeStart) < minWorkTime) {
                        response.pop()
                    } else {
                        response[response.length-1].push(time - timeBeforeStart)
                    }
                }
                continue
            }
            if (dat.exchange && dat.exchange === exchangeName) {
                if ((new RegExp('start.+' + marketName).test(dat.message))) {
                    response.push([time])
                    timeStart = time
                    continue
                }
                if ((new RegExp('retrying functions in.+' + marketName)).test(dat.message)) {
                    if ((time - timeStart) < minWorkTime) {
                        response.pop()
                    } else {
                        response[response.length-1].push(time)
                    }
                    // continue
                }
            }
        }
        return response;
    } catch (err) {
        logger.error(err.stack);
        throw new InternalServerError()
    }
}
