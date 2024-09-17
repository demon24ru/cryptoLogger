import { exec as noPromisifyExec } from 'child_process';

export function exec(cmd: string) {
    return new Promise((resolve, reject) => {
        noPromisifyExec(cmd, (error, stdout, stderr) => {
            if (error) return reject(error)
            if (stderr) return reject(stderr)
            resolve(stdout)
        })
    })
}
