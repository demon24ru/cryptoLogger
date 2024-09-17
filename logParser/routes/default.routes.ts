import { Router } from 'express';
import {defaultResponse} from '../middleware/response';
import {logParser} from '../controllers/log_parser.controller';

const router = Router();

router.get(
    '/log',
    defaultResponse(logParser)
);


export default router;
