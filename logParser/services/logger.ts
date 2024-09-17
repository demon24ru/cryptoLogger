import { clc, yellow } from '../helpers/cli-colors.util';
import { isPlainObject } from '../helpers/shared.utils';
import httpContext from 'express-http-context';
import * as config from '../config';

export type LogLevel = 'log' | 'error' | 'warn' | 'debug' | 'verbose';

export interface ConsoleLoggerOptions {
  /**
   * Enabled log levels.
   */
  logLevels?: LogLevel[];
}

const DEFAULT_LOG_LEVELS: LogLevel[] = [
  'log',
  'error',
  'warn',
  'debug',
  'verbose',
];

const LOG_LEVEL_VALUES: Record<LogLevel, number> = {
  debug: 0,
  verbose: 1,
  log: 2,
  warn: 3,
  error: 4,
};

function isLogLevelEnabled(
    targetLevel: LogLevel,
    logLevels: LogLevel[] | undefined,
): boolean {
  if (!logLevels || (Array.isArray(logLevels) && logLevels?.length === 0)) {
    return false;
  }
  if (logLevels.includes(targetLevel)) {
    return true;
  }
  const highestLogLevelValue = logLevels
      .map(level => LOG_LEVEL_VALUES[level])
      .sort((a, b) => b - a)?.[0];

  const targetLevelValue = LOG_LEVEL_VALUES[targetLevel];
  return targetLevelValue >= highestLogLevelValue;
}


export default class Logger {
  private static lastTimestampAt?: number;
  private originalContext?: string;

  constructor();
  // tslint:disable-next-line:unified-signatures
  constructor(context: string);
  // tslint:disable-next-line:unified-signatures
  constructor(context: string, options: ConsoleLoggerOptions);
  constructor(
      protected context?: string,
      protected options: ConsoleLoggerOptions = {},
  ) {
    if (!options?.logLevels) {
      this.options.logLevels = DEFAULT_LOG_LEVELS;
    }
    if (context) {
      this.originalContext = context;
    }
  }

  /**
   * Write a 'log' level log, if the configured level allows for it.
   * Prints to `stdout` with newline.
   */
  log(message: any, context?: string): void;
  log(message: any, ...optionalParams: [...any, string?]): void;
  log(message: any, ...optionalParams: any[]) {
    if (!this.isLevelEnabled('log')) {
      return;
    }
    const { messages, context } = this.getContextAndMessagesToPrint([
      message,
      ...optionalParams,
    ]);
    this.printMessages(messages, context, 'log');
  }

  /**
   * Write an 'error' level log, if the configured level allows for it.
   * Prints to `stderr` with newline.
   */
  error(message: any, stack?: string, context?: string): void;
  error(message: any, ...optionalParams: [...any, string?, string?]): void;
  error(message: any, ...optionalParams: any[]) {
    if (!this.isLevelEnabled('error')) {
      return;
    }
    const { messages, context, stack } =
        this.getContextAndStackAndMessagesToPrint([message, ...optionalParams]);

    this.printMessages(messages, context, 'error', 'stderr');
    // @ts-ignore
    this.printStackTrace(stack);
  }

  /**
   * Write a 'warn' level log, if the configured level allows for it.
   * Prints to `stdout` with newline.
   */
  warn(message: any, context?: string): void;
  warn(message: any, ...optionalParams: [...any, string?]): void;
  warn(message: any, ...optionalParams: any[]) {
    if (!this.isLevelEnabled('warn')) {
      return;
    }
    const { messages, context } = this.getContextAndMessagesToPrint([
      message,
      ...optionalParams,
    ]);
    this.printMessages(messages, context, 'warn');
  }

  /**
   * Write a 'debug' level log, if the configured level allows for it.
   * Prints to `stdout` with newline.
   */
  debug(message: any, context?: string): void;
  debug(message: any, ...optionalParams: [...any, string?]): void;
  debug(message: any, ...optionalParams: any[]) {
    if (!this.isLevelEnabled('debug')) {
      return;
    }
    const { messages, context } = this.getContextAndMessagesToPrint([
      message,
      ...optionalParams,
    ]);
    this.printMessages(messages, context, 'debug');
  }

  /**
   * Write a 'verbose' level log, if the configured level allows for it.
   * Prints to `stdout` with newline.
   */
  verbose(message: any, context?: string): void;
  verbose(message: any, ...optionalParams: [...any, string?]): void;
  verbose(message: any, ...optionalParams: any[]) {
    if (!this.isLevelEnabled('verbose')) {
      return;
    }
    const { messages, context } = this.getContextAndMessagesToPrint([
      message,
      ...optionalParams,
    ]);
    this.printMessages(messages, context, 'verbose');
  }

  /**
   * Set log levels
   * @param levels log levels
   */
  setLogLevels(levels: LogLevel[]) {
    if (!this.options) {
      this.options = {};
    }
    this.options.logLevels = levels;
  }

  /**
   * Set logger context
   * @param context context
   */
  setContext(context: string) {
    this.context = context;
  }

  /**
   * Resets the logger context to the value that was passed in the constructor.
   */
  resetContext() {
    this.context = this.originalContext;
  }

  isLevelEnabled(level: LogLevel): boolean {
    const logLevels = this.options?.logLevels;
    return isLogLevelEnabled(level, logLevels);
  }

  protected getTimestamp(): string {
    const localeStringOptions = {
      year: 'numeric',
      hour: 'numeric',
      minute: 'numeric',
      second: 'numeric',
      day: '2-digit',
      month: '2-digit',
    };
    return new Date(Date.now()).toLocaleString(
        undefined,
        localeStringOptions as Intl.DateTimeFormatOptions,
    );
  }

  protected formatMessage(): string {
    const user = httpContext.get('user');
    const method = httpContext.get('method');
    const status = httpContext.get('status');
    const url = httpContext.get('url');

    if (httpContext.get('ts'))
      Logger.lastTimestampAt = 0;

    const messageArr: string[] = [];
    if (method) {
      let met: string = method;
      if (status)
        met = `${method} [${status}]`;
      messageArr.push(`${met}`);
    }
    if (user) {
      messageArr.push(`User: ${JSON.stringify(user)}`);
    }
    if (url) {
      messageArr.push(`${url}`);
    }
    return messageArr.length ? ` ${messageArr.join(' -> ')} ` : '';
  }

  protected printMessages(
      messages: unknown[],
      context = '',
      logLevel: LogLevel = 'log',
      writeStreamType?: 'stdout' | 'stderr',
  ) {
    const color = this.getColorByLogLevel(logLevel);
    messages.forEach(message => {
      const output = isPlainObject(message)
          ? `${color('Object:')}\n${JSON.stringify(
              message,
              (key, value) =>
                  typeof value === 'bigint' ? value.toString() : value,
              2,
          )}\n`
          : color(message as string);

      const pidMessage = color(`[${config.PACKAGE_NAME}] ${process.pid}  - `);
      const contextMessage = context ? yellow(`[${context}] `) : '';
      const routData = color(this.formatMessage());
      const timestampDiff = this.updateAndGetTimestampDiff();
      const formattedLogLevel = color(logLevel.toUpperCase().padStart(7, ' '));
      const computedMessage = `${pidMessage}${this.getTimestamp()} ${formattedLogLevel} ${contextMessage}${routData}${output}${timestampDiff}\n`;

      process[writeStreamType ?? 'stdout'].write(computedMessage);
    });
  }

  protected printStackTrace(stack: string) {
    if (!stack) {
      return;
    }
    process.stderr.write(`${stack}\n`);
  }

  private updateAndGetTimestampDiff(): string {
    const result = Logger.lastTimestampAt
        ? yellow(` +${Date.now() - Logger.lastTimestampAt}ms`)
        : '';
    Logger.lastTimestampAt = Date.now();
    return result;
  }

  private getContextAndMessagesToPrint(args: unknown[]) {
    if (args?.length <= 1) {
      return { messages: args, context: this.context };
    }
    const lastElement = args[args.length - 1];
    const isContext = typeof lastElement === 'string';
    if (!isContext) {
      return { messages: args, context: this.context };
    }
    return {
      context: lastElement as string,
      messages: args.slice(0, args.length - 1),
    };
  }

  private getContextAndStackAndMessagesToPrint(args: unknown[]) {
    const { messages, context } = this.getContextAndMessagesToPrint(args);
    if (messages?.length <= 1) {
      return { messages, context };
    }
    const lastElement = messages[messages.length - 1];
    const isStack = typeof lastElement === 'string';
    if (!isStack) {
      return { messages, context };
    }
    return {
      stack: lastElement as string,
      messages: messages.slice(0, messages.length - 1),
      context,
    };
  }

  private getColorByLogLevel(level: LogLevel) {
    switch (level) {
      case 'debug':
        return clc.magentaBright;
      case 'warn':
        return clc.yellow;
      case 'error':
        return clc.red;
      case 'verbose':
        return clc.cyanBright;
      default:
        return clc.green;
    }
  }
}
