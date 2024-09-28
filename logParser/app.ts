import {NODE_ENV, PORT} from './config';
import cors from 'cors';
import compression from 'compression';
import express, { NextFunction, Request, Response } from 'express';
import httpContext from 'express-http-context';
import { errorMiddleware } from './middleware/errors';
import routes from './routes';
import Logger from './services/logger';
import { notFoundRouts } from './middleware/response';
import {parseMarkets} from './store';


const logger: Logger = new Logger('App');

parseMarkets().then()

const app = express();

app.use(compression());

app.use(httpContext.middleware);
app.use((req: Request, res: Response, next: NextFunction) => {
    httpContext.ns.bindEmitter(req);
    httpContext.ns.bindEmitter(res);
    httpContext.set('method', req.method);
    httpContext.set('url', req.url);
    httpContext.set('ts', true);
    logger.log('');
    httpContext.set('ts', false);
    next();
});

app.use(cors({ origin: '*' }));

app.use(express.json());

app.use('', routes);

// app.use(express.static('public'));

app.use(notFoundRouts);

app.use(errorMiddleware);

if (NODE_ENV !== 'test') {
    try {
        logger.log(`Start ${NODE_ENV} mode`);
        app.listen(PORT, () => {
            logger.log(`Start server on port ${PORT}.`);
        });
    } catch ({ message }) {
        logger.error(message);
        process.exit(1);
    }
}
