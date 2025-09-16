#ifndef FILE_OPERATIONS_H
#define FILE_OPERATIONS_H

#include <string>
#include "http.h" // For HttpRequest

// Fungsi untuk menangani permintaan API file
void handle_api_request(int client_socket, const HttpRequest& req, const std::string& root_directory);

// Fungsi untuk membuat daftar isi direktori dalam format JSON
std::string list_directory_json(const std::string& path);

// Fungsi untuk menangani unggahan file
void handle_upload(int client_socket, const HttpRequest& req, const std::string& root_directory);

// Fungsi untuk membuat folder baru
void handle_new_folder(int client_socket, const HttpRequest& req, const std::string& root_directory);

// Fungsi untuk mengganti nama file/folder
void handle_rename(int client_socket, const HttpRequest& req, const std::string& root_directory);

// Fungsi untuk menghapus file/folder
void handle_delete(int client_socket, const HttpRequest& req, const std::string& root_directory);

// Fungsi untuk memindahkan/menyalin file
void handle_move(int client_socket, const HttpRequest& req, const std::string& root_directory);

// Fungsi untuk membuat arsip zip
void handle_zip(int client_socket, const HttpRequest& req, const std::string& root_directory);


#endif // FILE_OPERATIONS_H
