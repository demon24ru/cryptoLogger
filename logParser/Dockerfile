FROM node:20.9-alpine

RUN mkdir -p /home/app
WORKDIR /home/app

ARG NODE_ENV
ENV NODE_ENV $NODE_ENV
ENV PATH /home/app/node_modules/.bin:$PATH

COPY . .

RUN apk update && apk add --no-cache --virtual --update \
  make \
  curl

RUN npm install --force
RUN rimraf build && npm run build

ENV PORT 4000
EXPOSE $PORT

ENTRYPOINT [ "npm", "run" ,"start:serv" ]
