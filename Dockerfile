FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY backend ./backend
COPY frontend ./frontend
COPY database.sql ./

WORKDIR /app/backend
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o wifi-management .

FROM alpine:3.22

WORKDIR /app/backend

COPY --from=builder /app/backend/wifi-management ./wifi-management
COPY --from=builder /app/frontend /app/frontend
COPY --from=builder /app/database.sql /app/database.sql

RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./wifi-management"]
