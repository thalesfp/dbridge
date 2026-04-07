// Sample data for testing dbridge MongoDB driver
// This script runs as MONGO_INITDB_ROOT_USERNAME against MONGO_INITDB_DATABASE

db = db.getSiblingDB('dbridge_test');

db.createUser({
  user: 'dbridge_test',
  pwd: 'dbridge_test',
  roles: [{ role: 'readWrite', db: 'dbridge_test' }]
});

db.createCollection('users');
db.createCollection('orders');

db.users.insertMany([
  { email: 'alice@example.com', name: 'Alice Johnson', active: true, age: 28, created_at: new Date('2024-01-15T10:00:00Z') },
  { email: 'bob@example.com', name: 'Bob Smith', active: true, age: 34, created_at: new Date('2024-02-20T11:30:00Z') },
  { email: 'charlie@example.com', name: 'Charlie Brown', active: false, age: 42, created_at: new Date('2024-03-10T09:15:00Z') },
  { email: 'diana@example.com', name: 'Diana Prince', active: true, age: 31, created_at: new Date('2024-04-05T14:45:00Z') },
  { email: 'eve@example.com', name: 'Eve Wilson', active: true, age: 25, created_at: new Date('2024-05-12T16:20:00Z') },
]);

db.users.createIndex({ email: 1 }, { unique: true });
db.users.createIndex({ active: 1, age: -1 });

db.orders.insertMany([
  { user_email: 'alice@example.com', product: 'Widget A', amount: 29.99, status: 'completed', created_at: new Date('2024-06-01T10:00:00Z') },
  { user_email: 'alice@example.com', product: 'Widget B', amount: 49.99, status: 'completed', created_at: new Date('2024-06-15T11:00:00Z') },
  { user_email: 'bob@example.com', product: 'Gadget X', amount: 99.99, status: 'pending', created_at: new Date('2024-07-01T09:00:00Z') },
  { user_email: 'charlie@example.com', product: 'Widget A', amount: 29.99, status: 'cancelled', created_at: new Date('2024-07-10T14:00:00Z') },
  { user_email: 'diana@example.com', product: 'Gadget Y', amount: 149.99, status: 'completed', created_at: new Date('2024-08-01T16:00:00Z') },
]);

db.orders.createIndex({ user_email: 1 });
db.orders.createIndex({ status: 1 });
