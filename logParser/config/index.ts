import dotenv from 'dotenv';
import * as pkg from '../package.json'

dotenv.config();

export const PACKAGE_NAME: string = pkg.name;

export const LOG_FILE_MASK_NAME: string = process.env.LOG_FILE_MASK_NAME as string

export const NODE_ENV: string = process.env.NODE_ENV || 'development';

export const PORT: number = +process.env.PORT || 4000;

export const NO_COLOR_LOG: string = process.env.NO_COLOR_LOG as string;
