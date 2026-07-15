CREATE TABLE IF NOT EXISTS address_books (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL UNIQUE,
  description TEXT,
  owner_user_id BIGINT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (owner_user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS address_book_entries (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  address_book_id BIGINT NOT NULL,
  device_id BIGINT NOT NULL,
  alias VARCHAR(255),
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_address_book_device (address_book_id, device_id),
  FOREIGN KEY (address_book_id) REFERENCES address_books(id),
  FOREIGN KEY (device_id) REFERENCES devices(id)
);
