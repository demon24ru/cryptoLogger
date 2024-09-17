import { Router } from 'express';
import defaultRouter from './default.routes';

const router = Router();

router.use('/', defaultRouter);

export default router;
