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
        const dat = JSON.parse(fs.readFileSync('./configs/' + file, 'utf8').toString());
        const config = file.match(/\d{1,}/)[0]
        for (const d of dat.exchanges) {
            if (d.name === 'kucoin') {
                if (!markets.kucoin)
                    markets.kucoin = {}
                for (const market of d.markets) {
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
                        throw new Error(`Market ${market.id}, result.code not 200`);
                    }

                    markets.kucoin[market.id.split('-').join('')] = {
                        priceIncrement: Number(result.data.priceIncrement),
                        multiplier: 1,
                        config
                    }
                    await sleep(50);
                    logger.log(`Load kucoin ${market.id}`)
                }
            } else if (d.name === 'kucoinFutures') {
                if (!markets.kucoin)
                    markets.kucoin = {}
                for (const market of d.markets) {
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
                        throw new Error(`Market ${market.id}, result.code not 200`);
                    }

                    markets.kucoin[`${market.id.slice(0, -1)}F`] = {
                        priceIncrement: result.data.tickSize,
                        multiplier: result.data.multiplier,
                        config
                    }
                    await sleep(50);
                    logger.log(`Load kucoinFutures ${market.id}`)
                }
            }
        }
    }
}

export default markets
