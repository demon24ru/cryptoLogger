import fs from 'fs';
import Logger from '../services/logger';


const logger: Logger = new Logger('MarketsParser');

const markets: {
    [exchange: string]: {
        [market: string]: {
            priceIncrement: number
            multiplier: number
            config: string
        }
    }
} = {}

function sleep(ms) {
    return new Promise((resolve) => {
        setTimeout(resolve, ms);
    });
}

export async function parseMarkets(): Promise<void> {
    const files = fs.readdirSync('./configs')
        .filter((file) => (file.indexOf('.') !== 0) && (file.slice(-5) === '.json'))

    for (const file of files) {
        try {
            const dat = JSON.parse(fs.readFileSync('./configs/' + file, 'utf8').toString());
        } catch (e) {
            logger.error(`Load file ${file}`, e.message, e.stack)
            continue
        }
        const config = file.match(/\d{1,}/)[0]
        for (const d of dat.exchanges) {
            switch (d.name) {
                case 'kucoin':
                    if (!markets.kucoin)
                        markets.kucoin = {}
                    for (const market of d.markets) {
                        try {
                            const resf = await fetch(`https://www.kucoin.com/_api/trade-front/trade-basic-info?symbol=${market.id}`, {method: 'GET'});
                            if (!resf.ok) {
                                continue
                            }
                            const result: any = await resf.json()

                            if (result.code === '100000') {
                                continue
                            }
                            if (result.code !== '200') {
                                logger.error(market.id, result)
                                throw new Error(`Kucoin Spot Market ${market.id}, result.code not 200`);
                            }

                            markets.kucoin[market.id.split('-').join('')] = {
                                priceIncrement: Number(result.data.priceIncrement),
                                multiplier: 1,
                                config
                            }
                            await sleep(50);
                            logger.log(`Load kucoin ${market.id}`)
                        } catch (e) {
                            logger.error(`Load kucoinFutures ${market.id}`, e.message, e.stack)
                        }
                    }
                    break
                case 'kucoinFutures':
                    if (!markets.kucoin)
                        markets.kucoin = {}
                    for (const market of d.markets) {
                        try {
                            const resf = await fetch(`https://www.kucoin.com/_api_kumex/web-front/contracts/${market.id}`, {method: 'GET'});
                            if (!resf.ok) {
                                continue
                            }
                            const result: any = await resf.json()

                            if (result.code === '100000') {
                                continue
                            }
                            if (result.code !== '200') {
                                logger.error(market.id, result)
                                throw new Error(`Kucoin Futures Market ${market.id}, result.code not 200`);
                            }

                            markets.kucoin[`${market.id.slice(0, -1)}F`] = {
                                priceIncrement: result.data.tickSize,
                                multiplier: result.data.multiplier,
                                config
                            }
                            await sleep(50);
                            logger.log(`Load kucoinFutures ${market.id}`)
                        } catch (e) {
                            logger.error(`Load kucoinFutures ${market.id}`, e.message, e.stack)
                        }
                    }
                    break
                case 'bybit':
                    if (!markets.bybit)
                        markets.bybit = {}
                    for (const market of d.markets) {
                        try {
                            // tslint:disable-next-line:max-line-length
                            const resf = await fetch(`https://api.bybit.com/v5/market/instruments-info?category=spot&symbol=${market.id}`, {method: 'GET'});
                            if (!resf.ok) {
                                continue
                            }
                            const result: any = await resf.json()

                            if (result.retCode !== 0) {
                                logger.error(market.id, result)
                                throw new Error(`Bybit Spot Market ${market.id}, result.code not 200`);
                            }

                            markets.bybit[market.id] = {
                                priceIncrement: Number(result.result.list[0].priceFilter?.tickSize),
                                multiplier: 1,
                                config
                            }
                            await sleep(50);
                            logger.log(`Load Bybit ${market.id}`)
                        } catch (e) {
                            logger.error(`Load Bybit ${market.id}`, e.message, e.stack)
                        }
                    }
                    break
                case 'bybitFutures':
                    if (!markets.bybit)
                        markets.bybit = {}
                    for (const market of d.markets) {
                        try {
                            const resf = await fetch(`https://api.bybit.com/v5/market/instruments-info?category=linear&symbol=${market.id}`, {method: 'GET'});
                            if (!resf.ok) {
                                continue
                            }
                            const result: any = await resf.json()

                            if (result.retCode !== 0) {
                                logger.error(market.id, result)
                                throw new Error(`Bybit Futures Market ${market.id}, result.code not 200`);
                            }

                            markets.bybit[`${market.id}F`] = {
                                priceIncrement: Number(result.result.list[0].priceFilter?.tickSize),
                                multiplier: 1,
                                config
                            }
                            await sleep(50);
                            logger.log(`Load BybitFutures ${market.id}`)
                        } catch (e) {
                            logger.error(`Load BybitFutures ${market.id}`, e.message, e.stack)
                        }
                    }
                    break
            }
        }
    }
}

export default markets
