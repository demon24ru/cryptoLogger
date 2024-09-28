import { Router } from 'express';
import {defaultResponse} from '../middleware/response';
import {logParser} from '../controllers/log_parser.controller';
import {marketsParser} from '../controllers/markets_parser.controller';

const router = Router();

router.get(
    '/log',
    defaultResponse(logParser)
);

router.get(
    '/markets',
    defaultResponse(marketsParser)
);


export default router;
