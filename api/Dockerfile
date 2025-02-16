FROM node:20-alpine3.21 AS development
WORKDIR /usr/src/app

# Copy package files
COPY package*.json ./

# Install dependencies inside the container
RUN npm install && npm rebuild better-sqlite3

CMD [ "npm", "run", "start:dev"]