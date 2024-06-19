ARG NODE_VERSION=22.3

FROM node:${NODE_VERSION}-alpine
RUN mkdir -p /opt/app
WORKDIR /opt/app
COPY package.json .
RUN npm install
COPY . . 
CMD [ "node", "index.js"]
 