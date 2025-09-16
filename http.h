#ifndef HTTP_H
#define HTTP_H

#include <iostream>
#include <string>
#include <vector>
#include <unordered_map>

// Untuk networking
#include <sys/socket.h>
#include <netinet/in.h>
#include <unistd.h>

// Struktur untuk menyimpan detail permintaan HTTP
struct HttpRequest {
    std::string method;
    std::string path;
    std::unordered_map<std::string, std::string> headers;
    std::vector<char> body; // Gunakan vector<char> untuk data biner
};

// Fungsi untuk memulai server
void start_server(int port, const std::string& root_directory);

// Fungsi untuk menangani koneksi klien
void handle_connection(int client_socket, const std::string& root_directory);

// Fungsi untuk parsing permintaan HTTP
HttpRequest parse_request(int client_socket);

// Fungsi untuk mengirim respons HTTP
void send_response(int client_socket, const std::string& status, const std::string& content_type, const std::string& body);
void send_response_with_headers(int client_socket, const std::string& status, const std::vector<std::string>& headers, const std::string& body);


#endif // HTTP_H
