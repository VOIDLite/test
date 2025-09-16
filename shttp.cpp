#include "http.h"
#include "file_operations.h"
#include <iostream>
#include <string>
#include <vector>
#include <filesystem>
#include <thread>
#include <fstream>
#include <sstream>

// Helper untuk mendapatkan tipe MIME berdasarkan ekstensi file
std::string get_mime_type(const std::string& path) {
    if (path.find(".html") != std::string::npos) return "text/html";
    if (path.find(".css") != std::string::npos) return "text/css";
    if (path.find(".js") != std::string::npos) return "application/javascript";
    if (path.find(".png") != std::string::npos) return "image/png";
    if (path.find(".jpg") != std::string::npos) return "image/jpeg";
    if (path.find(".jpeg") != std::string::npos) return "image/jpeg";
    if (path.find(".gif") != std::string::npos) return "image/gif";
    return "application/octet-stream"; // Tipe default
}

std::string generate_file_browser_html() {
    return R"HTML(
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>File Browser</title>
    <link rel="stylesheet" href="/frontend/common.css">
    <link rel="stylesheet" href="/frontend/file-browser/file-browser.css">
    <script src="/frontend/common.js"></script>
    <script src="/frontend/file-browser/file-browser.js"></script>
</head>
<body onload="onPageLoad()">
    <h1>File Browser</h1>
    <div id="actionbar">
        <span id="path">/</span>
        <div id="loader" style="visibility: hidden;"></div>
        <button id="upload-button" onclick="doUpload()">
            <span class="button__text">UPLOAD FILES</span>
            <span class="button__progress"></span>
        </button>
        <input type="file" id="files" multiple onchange="onUploadFilesSelected(this)" style="display: none;">
        <button onclick="onNewFolderClick()">New Folder</button>
        <button onclick="onNewFileClick()">New File</button>
        <button id="select-button" onclick="onSelectModeClick(this)">SELECT</button>
        <button id="rename-button" onclick="onRenameClick()" disabled>RENAME</button>
        <button id="cut-button" onclick="onPushToClipboardClick('move')" disabled>CUT</button>
        <button id="copy-button" onclick="onPushToClipboardClick('copy')" disabled>COPY</button>
        <button id="delete-button" onclick="onDeleteClick()" disabled>DELETE</button>
        <button id="paste-button" onclick="onPasteClick()" disabled>PASTE</button>
        <button id="zip-button" onclick="onZipClick()" disabled>ZIP</button>
    </div>
    <div id="view-mode">
        <label><input type="radio" name="view" value="list" id="radioList" onchange="viewModeChange(this)"> List</label>
        <label><input type="radio" name="view" value="table" id="radioTable" onchange="viewModeChange(this)"> Table</label>
        <label><input type="radio" name="view" value="grid" id="radioGrid" onchange="viewModeChange(this)"> Grid</label>
    </div>
    <div id="files-container" ondragover="dragOverHandler(event);" ondrop="dropHandler(event);">
        <!-- File list will be rendered here by JavaScript -->
    </div>
    <div id="context-menu" class="context-menu">
        <ul>
            <li id="mo-open-in-new-tab">Open in new tab</li>
            <li id="mo-edit-as-text">Edit as text</li>
            <li id="mo-rename">Rename</li>
            <li id="mo-delete">Delete</li>
            <li id="mo-download">Download</li>
            <li id="mo-zip">Zip selected</li>
            <li id="mo-copy-link">Copy link</li>
        </ul>
    </div>
     <div id="main-menu" class="context-menu">
        <ul>
            <li id="mm-database">Database browser</li>
            <li id="mm-login">Login</li>
            <li id="mm-logout">Logout</li>
        </ul>
    </div>
</body>
</html>
)HTML";
}

// Helper untuk escape JSON string
std::string escape_json(const std::string &s) {
    std::stringstream o;
    for (auto c = s.cbegin(); c != s.cend(); c++) {
        switch (*c) {
            case '"': o << "\\\""; break;
            case '\\': o << "\\\\"; break;
            case '\b': o << "\\b"; break;
            case '\f': o << "\\f"; break;
            case '\n': o << "\\n"; break;
            case '\r': o << "\\r"; break;
            case '\t': o << "\\t"; break;
            default:
                if ('\x00' <= *c && *c <= '\x1f') {
                    o << "\\u" << std::hex << std::setw(4) << std::setfill('0') << (int)*c;
                } else {
                    o << *c;
                }
        }
    }
    return o.str();
}


std::string list_directory_json(const std::string& path) {
    std::stringstream json_stream;
    json_stream << "[";
    bool first = true;

    try {
        for (const auto& entry : std::filesystem::directory_iterator(path)) {
            if (!first) {
                json_stream << ",";
            }
            first = false;

            auto last_write_time = std::filesystem::last_write_time(entry);
            auto sctp = std::chrono::time_point_cast<std::chrono::system_clock::duration>(last_write_time - std::filesystem::file_time_type::clock::now() + std::chrono::system_clock::now());
            std::time_t cftime = std::chrono::system_clock::to_time_t(sctp);

            json_stream << "{";
            json_stream << "\"name\":\"" << escape_json(entry.path().filename().string()) << "\",";
            json_stream << "\"directory\":" << (entry.is_directory() ? "true" : "false") << ",";
            json_stream << "\"length\":" << (entry.is_directory() ? 0 : entry.file_size()) << ",";
            json_stream << "\"modified\":" << cftime * 1000; // a JS-friendly timestamp in milliseconds
            json_stream << "}";
        }
    } catch (const std::filesystem::filesystem_error& e) {
        std::cerr << "Error listing directory: " << e.what() << std::endl;
        // Return empty list on error
    }

    json_stream << "]";
    return json_stream.str();
}

void handle_upload(int client_socket, const HttpRequest& req, const std::string& root_directory) {
    // Dapatkan path dari query string
    std::string upload_path_str = "/";
    size_t path_pos = req.path.find("path=");
    if (path_pos != std::string::npos) {
        upload_path_str = req.path.substr(path_pos + 5);
        // URL Decode (versi sederhana)
        std::string decoded_path;
        for (size_t i = 0; i < upload_path_str.length(); ++i) {
            if (upload_path_str[i] == '%' && i + 2 < upload_path_str.length()) {
                std::string hex = upload_path_str.substr(i + 1, 2);
                decoded_path += static_cast<char>(std::strtol(hex.c_str(), nullptr, 16));
                i += 2;
            } else if (upload_path_str[i] == '+') {
                decoded_path += ' ';
            } else {
                decoded_path += upload_path_str[i];
            }
        }
        upload_path_str = decoded_path;
    }

    std::filesystem::path final_upload_dir = root_directory;
    final_upload_dir /= upload_path_str;

    // Dapatkan boundary dari header Content-Type
    std::string content_type = req.headers.at("Content-Type");
    std::string boundary = "--" + content_type.substr(content_type.find("boundary=") + 9);

    // Parsing body multipart
    std::string body_str(req.body.begin(), req.body.end());
    size_t start_pos = 0;
    while ((start_pos = body_str.find(boundary, start_pos)) != std::string::npos) {
        start_pos += boundary.length();
        if (body_str.substr(start_pos, 2) == "--") {
            break; // Batas akhir
        }
        start_pos += 2; // Lewati \r\n

        size_t headers_end = body_str.find("\r\n\r\n", start_pos);
        if (headers_end == std::string::npos) continue;

        std::string headers = body_str.substr(start_pos, headers_end - start_pos);

        // Ekstrak nama file
        size_t filename_pos = headers.find("filename=\"");
        if (filename_pos == std::string::npos) continue;

        filename_pos += 10;
        size_t filename_end = headers.find("\"", filename_pos);
        std::string filename = headers.substr(filename_pos, filename_end - filename_pos);

        if (filename.empty()) continue;

        size_t content_start = headers_end + 4;
        size_t content_end = body_str.find(boundary, content_start);
        if (content_end == std::string::npos) break;

        // Kurangi 2 untuk \r\n sebelum boundary berikutnya
        std::string file_content = body_str.substr(content_start, content_end - content_start - 2);

        // Simpan file
        std::filesystem::path file_path = final_upload_dir / filename;
        std::ofstream outfile(file_path, std::ios::binary);
        if (outfile) {
            outfile.write(file_content.c_str(), file_content.length());
            outfile.close();
            std::cout << "Saved file: " << file_path << std::endl;
        } else {
             std::cerr << "Failed to open file for writing: " << file_path << std::endl;
             send_response(client_socket, "500 Internal Server Error", "text/plain", "Failed to save file.");
             return;
        }
    }

    send_response(client_socket, "204 No Content", "text/plain", "");
}

// Helper untuk parsing x-www-form-urlencoded
std::unordered_map<std::string, std::string> parse_form_data(const std::string& body) {
    std::unordered_map<std::string, std::string> data;
    std::stringstream ss(body);
    std::string pair;
    while (std::getline(ss, pair, '&')) {
        size_t eq_pos = pair.find('=');
        if (eq_pos != std::string::npos) {
            std::string key = pair.substr(0, eq_pos);
            std::string value = pair.substr(eq_pos + 1);
            // Quick and dirty URL decode
            std::string decoded_value;
            for (size_t i = 0; i < value.length(); ++i) {
                if (value[i] == '%' && i + 2 < value.length()) {
                    std::string hex = value.substr(i + 1, 2);
                    decoded_value += static_cast<char>(std::strtol(hex.c_str(), nullptr, 16));
                    i += 2;
                } else if (value[i] == '+') {
                    decoded_value += ' ';
                } else {
                    decoded_value += value[i];
                }
            }
            data[key] = decoded_value;
        }
    }
    return data;
}

void handle_new_folder(int client_socket, const HttpRequest& req, const std::string& root_directory) {
    std::string body_str(req.body.begin(), req.body.end());
    auto form_data = parse_form_data(body_str);

    if (form_data.count("path") && form_data.count("name")) {
        std::filesystem::path new_dir_path = root_directory;
        new_dir_path /= form_data["path"];
        new_dir_path /= form_data["name"];

        try {
            if (std::filesystem::create_directory(new_dir_path)) {
                send_response(client_socket, "204 No Content", "text/plain", "");
            } else {
                send_response(client_socket, "400 Bad Request", "text/plain", "Could not create directory.");
            }
        } catch (const std::filesystem::filesystem_error& e) {
            send_response(client_socket, "500 Internal Server Error", "text/plain", e.what());
        }
    } else {
        send_response(client_socket, "400 Bad Request", "text/plain", "Missing path or name parameter.");
    }
}

void handle_rename(int client_socket, const HttpRequest& req, const std::string& root_directory) {
    std::string body_str(req.body.begin(), req.body.end());
    auto form_data = parse_form_data(body_str);

    if (form_data.count("path") && form_data.count("name")) {
        std::filesystem::path old_path = root_directory;
        old_path /= form_data["path"];

        std::filesystem::path new_path = old_path.parent_path() / form_data["name"];

        try {
            std::filesystem::rename(old_path, new_path);
            send_response(client_socket, "204 No Content", "text/plain", "");
        } catch (const std::filesystem::filesystem_error& e) {
            send_response(client_socket, "500 Internal Server Error", "text/plain", e.what());
        }
    } else {
        send_response(client_socket, "400 Bad Request", "text/plain", "Missing path or name parameter.");
    }
}

void handle_delete(int client_socket, const HttpRequest& req, const std::string& root_directory) {
    std::string body_str(req.body.begin(), req.body.end());

    // Parser JSON super sederhana
    try {
        // Ekstrak path
        size_t path_pos = body_str.find("\"path\":\"");
        path_pos += 8;
        size_t path_end = body_str.find("\"", path_pos);
        std::string path = body_str.substr(path_pos, path_end - path_pos);

        // Ekstrak files
        size_t files_pos = body_str.find("\"files\":[");
        files_pos += 9;
        size_t files_end = body_str.find("]", files_pos);
        std::string files_str = body_str.substr(files_pos, files_end - files_pos);

        std::filesystem::path base_path = root_directory;
        base_path /= path;

        std::stringstream ss(files_str);
        std::string filename;
        while(std::getline(ss, filename, ',')) {
            // Hapus tanda kutip
            if (!filename.empty() && filename.front() == '"') filename.erase(0, 1);
            if (!filename.empty() && filename.back() == '"') filename.pop_back();

            std::filesystem::path file_to_delete = base_path / filename;
            std::filesystem::remove_all(file_to_delete); // Hapus file atau direktori secara rekursif
        }
        send_response(client_socket, "204 No Content", "text/plain", "");

    } catch (const std::exception& e) {
        send_response(client_socket, "400 Bad Request", "text/plain", "Invalid JSON body.");
    }
}

void handle_move(int client_socket, const HttpRequest& req, const std::string& root_directory) {
    std::string body_str(req.body.begin(), req.body.end());

    try {
        // Ekstrak action
        size_t action_pos = body_str.find("\"action\":\"");
        action_pos += 10;
        size_t action_end = body_str.find("\"", action_pos);
        std::string action = body_str.substr(action_pos, action_end - action_pos);

        // Ekstrak path (destination)
        size_t path_pos = body_str.find("\"path\":\"");
        path_pos += 8;
        size_t path_end = body_str.find("\"", path_pos);
        std::string dest_path_str = body_str.substr(path_pos, path_end - path_pos);

        // Ekstrak files
        size_t files_pos = body_str.find("\"files\":[");
        files_pos += 9;
        size_t files_end = body_str.find("]", files_pos);
        std::string files_str = body_str.substr(files_pos, files_end - files_pos);

        std::filesystem::path dest_dir = root_directory;
        dest_dir /= dest_path_str;

        std::stringstream ss(files_str);
        std::string src_path_str;
        while(std::getline(ss, src_path_str, ',')) {
            if (!src_path_str.empty() && src_path_str.front() == '"') src_path_str.erase(0, 1);
            if (!src_path_str.empty() && src_path_str.back() == '"') src_path_str.pop_back();

            std::filesystem::path src_path = root_directory;
            src_path /= src_path_str;

            std::filesystem::path dest_path = dest_dir / src_path.filename();

            if (action == "copy") {
                 std::filesystem::copy(src_path, dest_path, std::filesystem::copy_options::recursive);
            } else { // move
                 std::filesystem::rename(src_path, dest_path);
            }
        }
        send_response(client_socket, "204 No Content", "text/plain", "");

    } catch (const std::exception& e) {
         send_response(client_socket, "400 Bad Request", "text/plain", "Invalid JSON body for move/copy.");
    }
}

void handle_zip(int client_socket, const HttpRequest& req, const std::string& root_directory) {
    std::string body_str(req.body.begin(), req.body.end());
    auto form_data = parse_form_data(body_str);

    if (form_data.count("path") && form_data.count("files")) {
        std::filesystem::path base_path = root_directory;
        base_path /= form_data["path"];

        std::string files_json = form_data["files"];
        // Hapus kurung siku
        files_json.erase(0, 1);
        files_json.pop_back();

        std::string temp_zip_name = "archive.zip";
        std::filesystem::path temp_zip_path = std::filesystem::temp_directory_path() / temp_zip_name;

        std::string zip_command = "zip -j "; // -j untuk junk paths
        zip_command += temp_zip_path.string();
        zip_command += " ";

        std::stringstream ss(files_json);
        std::string filename;
        while(std::getline(ss, filename, ',')) {
            if (!filename.empty() && filename.front() == '"') filename.erase(0, 1);
            if (!filename.empty() && filename.back() == '"') filename.pop_back();
            zip_command += "'" + (base_path / filename).string() + "' ";
        }

        int result = system(zip_command.c_str());

        if (result == 0) {
            std::ifstream file(temp_zip_path, std::ios::binary);
            if (file) {
                std::stringstream buffer;
                buffer << file.rdbuf();
                std::string content = buffer.str();

                std::vector<std::string> headers;
                headers.push_back("Content-Type: application/zip");
                headers.push_back("Content-Disposition: attachment; filename=\"archive.zip\"");

                send_response_with_headers(client_socket, "200 OK", headers, content);
                file.close();
                std::filesystem::remove(temp_zip_path);
            } else {
                send_response(client_socket, "500 Internal Server Error", "text/plain", "Could not read temporary zip file.");
            }
        } else {
            send_response(client_socket, "500 Internal Server Error", "text/plain", "Failed to create zip file. Make sure 'zip' is installed.");
        }
    } else {
        send_response(client_socket, "400 Bad Request", "text/plain", "Missing path or files parameter for zip.");
    }
}


void handle_api_request(int client_socket, const HttpRequest& req, const std::string& root_directory) {
    // Path /api/file/list
    if (req.path.rfind("/api/file/list", 0) == 0) {
        // Ekstrak query param 'path'
        std::string query_path = "/";
        size_t path_pos = req.path.find("path=");
        if (path_pos != std::string::npos) {
            query_path = req.path.substr(path_pos + 5);
            // URL Decode
            // Simple decode for now. A proper library should be used for production.
            // This is a simplified version.
            std::string decoded_path;
            for (size_t i = 0; i < query_path.length(); ++i) {
                if (query_path[i] == '%' && i + 2 < query_path.length()) {
                    std::string hex = query_path.substr(i + 1, 2);
                    char c = static_cast<char>(std::strtol(hex.c_str(), nullptr, 16));
                    decoded_path += c;
                    i += 2;
                } else if (query_path[i] == '+') {
                    decoded_path += ' ';
                } else {
                    decoded_path += query_path[i];
                }
            }
             query_path = decoded_path;
        }

        std::filesystem::path full_path = root_directory + query_path;
        std::string json_response = list_directory_json(full_path.string());
        send_response(client_socket, "200 OK", "application/json", json_response);
    } else if (req.method == "PUT" && req.path.rfind("/api/file/upload", 0) == 0) {
        handle_upload(client_socket, req, root_directory);
    } else if (req.method == "POST" && req.path.rfind("/api/file/new-folder", 0) == 0) {
        handle_new_folder(client_socket, req, root_directory);
    } else if (req.method == "POST" && req.path.rfind("/api/file/rename", 0) == 0) {
        handle_rename(client_socket, req, root_directory);
    } else if (req.method == "DELETE" && req.path.rfind("/api/file/delete", 0) == 0) {
        handle_delete(client_socket, req, root_directory);
    } else if (req.method == "POST" && req.path.rfind("/api/file/move", 0) == 0) {
        handle_move(client_socket, req, root_directory);
    } else if (req.method == "POST" && req.path.rfind("/api/file/zip", 0) == 0) {
        handle_zip(client_socket, req, root_directory);
    }
    else {
        send_response(client_socket, "404 Not Found", "text/plain", "API endpoint not found.");
    }
}


// Fungsi untuk menampilkan penggunaan
void show_usage(const std::string& name) {
    std::cerr << "Usage: " << name << " [-p port] [-d directory]" << std::endl;
}

int main(int argc, char* argv[]) {
    int port = 8080;
    std::string root_directory = ".";

    // Parsing argumen baris perintah
    for (int i = 1; i < argc; ++i) {
        std::string arg = argv[i];
        if ((arg == "-p") && i + 1 < argc) {
            try {
                port = std::stoi(argv[++i]);
            } catch (const std::invalid_argument& e) {
                std::cerr << "Error: Invalid port number." << std::endl;
                show_usage(argv[0]);
                return 1;
            }
        } else if ((arg == "-d") && i + 1 < argc) {
            root_directory = argv[++i];
        } else {
            show_usage(argv[0]);
            return 1;
        }
    }

    // Ubah ke path absolut
    try {
        root_directory = std::filesystem::canonical(root_directory);
    } catch (const std::filesystem::filesystem_error& e) {
        std::cerr << "Error: Root directory not found: " << root_directory << std::endl;
        return 1;
    }

    std::cout << "Starting server on port " << port << " serving " << root_directory << std::endl;

    start_server(port, root_directory);

    return 0;
}

void start_server(int port, const std::string& root_directory) {
    int server_fd;
    struct sockaddr_in address;
    int opt = 1;
    int addrlen = sizeof(address);

    // Membuat soket file descriptor
    if ((server_fd = socket(AF_INET, SOCK_STREAM, 0)) == 0) {
        perror("socket failed");
        exit(EXIT_FAILURE);
    }

    // Mengatur opsi soket
    if (setsockopt(server_fd, SOL_SOCKET, SO_REUSEADDR | SO_REUSEPORT, &opt, sizeof(opt))) {
        perror("setsockopt");
        exit(EXIT_FAILURE);
    }
    address.sin_family = AF_INET;
    address.sin_addr.s_addr = INADDR_ANY;
    address.sin_port = htons(port);

    // Binding soket ke port
    if (bind(server_fd, (struct sockaddr *)&address, sizeof(address)) < 0) {
        perror("bind failed");
        exit(EXIT_FAILURE);
    }

    // Mendengarkan koneksi
    if (listen(server_fd, 10) < 0) {
        perror("listen");
        exit(EXIT_FAILURE);
    }

    std::cout << "Server listening on port " << port << "..." << std::endl;

    while (true) {
        int client_socket;
        if ((client_socket = accept(server_fd, (struct sockaddr *)&address, (socklen_t*)&addrlen)) < 0) {
            perror("accept");
            continue; // Jangan keluar, coba lagi
        }

        // Buat thread baru untuk setiap koneksi
        std::thread(handle_connection, client_socket, root_directory).detach();
    }
}

void handle_connection(int client_socket, const std::string& root_directory) {
    HttpRequest req = parse_request(client_socket);

    if (req.method.empty()) {
        close(client_socket);
        return;
    }

    std::cout << "Request: " << req.method << " " << req.path << std::endl;

    // Mencegah directory traversal
    std::filesystem::path requested_path = root_directory;

    // Hapus tanda '/' di awal path jika ada
    if (!req.path.empty() && req.path[0] == '/') {
        requested_path /= req.path.substr(1);
    } else {
        requested_path /= req.path;
    }

    std::filesystem::path canonical_path;
    try {
        canonical_path = std::filesystem::canonical(requested_path);
    } catch (const std::filesystem::filesystem_error& e) {
        std::cerr << "Error: Path not found: " << requested_path << std::endl;
        send_response(client_socket, "404 Not Found", "text/plain", "File not found.");
        close(client_socket);
        return;
    }

    // Cek apakah ini adalah permintaan API
    if (req.path.rfind("/api/", 0) == 0) {
        handle_api_request(client_socket, req, root_directory);
        close(client_socket);
        return;
    }

    // Verifikasi bahwa path yang diminta berada di dalam root directory
    std::string root_str = std::filesystem::canonical(root_directory).string();
    std::string path_str = canonical_path.string();
    if (path_str.rfind(root_str, 0) != 0) {
        send_response(client_socket, "403 Forbidden", "text/plain", "Access denied.");
        close(client_socket);
        return;
    }

    if (std::filesystem::is_regular_file(canonical_path)) {
        std::ifstream file(canonical_path, std::ios::binary);
        if (file) {
            std::stringstream buffer;
            buffer << file.rdbuf();
            std::string content = buffer.str();
            send_response(client_socket, "200 OK", get_mime_type(path_str), content);
        } else {
            send_response(client_socket, "500 Internal Server Error", "text/plain", "Could not read file.");
        }
    } else if (std::filesystem::is_directory(canonical_path)) {
        // Jika path adalah direktori, sajikan HTML file browser
        std::string html = generate_file_browser_html();
        send_response(client_socket, "200 OK", "text/html", html);
    } else {
        send_response(client_socket, "404 Not Found", "text/plain", "Resource not found.");
    }

    close(client_socket);
}

HttpRequest parse_request(int client_socket) {
    HttpRequest req;
    std::string request_str;
    char buffer[4096];
    int bytes_read;

    // Baca header terlebih dahulu
    while ((bytes_read = read(client_socket, buffer, 4095)) > 0) {
        buffer[bytes_read] = '\0';
        request_str += buffer;
        if (request_str.find("\r\n\r\n") != std::string::npos) {
            break;
        }
    }

    if (request_str.empty()) {
        return req;
    }

    // Parsing request line
    size_t first_line_end = request_str.find("\r\n");
    std::string request_line = request_str.substr(0, first_line_end);
    size_t method_end = request_line.find(' ');
    if (method_end != std::string::npos) {
        req.method = request_line.substr(0, method_end);
        size_t path_end = request_line.find(' ', method_end + 1);
        if (path_end != std::string::npos) {
            req.path = request_line.substr(method_end + 1, path_end - (method_end + 1));
        }
    }

    // Parsing headers
    size_t headers_start = first_line_end + 2;
    size_t headers_end = request_str.find("\r\n\r\n", headers_start);
    std::string headers_block = request_str.substr(headers_start, headers_end - headers_start);
    std::stringstream ss(headers_block);
    std::string header_line;
    while (std::getline(ss, header_line) && !header_line.empty() && header_line != "\r") {
        size_t colon_pos = header_line.find(':');
        if (colon_pos != std::string::npos) {
            std::string key = header_line.substr(0, colon_pos);
            std::string value = header_line.substr(colon_pos + 2); // +2 untuk spasi setelah ':'
            // hapus \r di akhir value
             if (!value.empty() && value.back() == '\r') {
                value.pop_back();
            }
            req.headers[key] = value;
        }
    }

    // Jika ada body, baca berdasarkan Content-Length
    if (req.headers.count("Content-Length")) {
        int content_length = std::stoi(req.headers["Content-Length"]);
        size_t body_start = headers_end + 4;

        std::vector<char> body_buffer;
        body_buffer.reserve(content_length);

        // Salin bagian body yang sudah terbaca dari buffer awal
        if (request_str.length() > body_start) {
            body_buffer.insert(body_buffer.end(), request_str.begin() + body_start, request_str.end());
        }

        // Baca sisa body
        int remaining = content_length - body_buffer.size();
        if (remaining > 0) {
            std::vector<char> remaining_buffer(remaining);
            int total_read = 0;
            while(total_read < remaining) {
                bytes_read = read(client_socket, remaining_buffer.data() + total_read, remaining - total_read);
                if (bytes_read <= 0) break;
                total_read += bytes_read;
            }
            body_buffer.insert(body_buffer.end(), remaining_buffer.begin(), remaining_buffer.begin() + total_read);
        }
        req.body = body_buffer;
    }

    return req;
}

void send_response(int client_socket, const std::string& status, const std::string& content_type, const std::string& body) {
    std::string response = "HTTP/1.1 " + status + "\r\n";
    response += "Content-Type: " + content_type + "\r\n";
    response += "Content-Length: " + std::to_string(body.length()) + "\r\n";
    response += "\r\n";
    response += body;

    send(client_socket, response.c_str(), response.length(), 0);
}

void send_response_with_headers(int client_socket, const std::string& status, const std::vector<std::string>& headers, const std::string& body) {
    std::string response = "HTTP/1.1 " + status + "\r\n";
    for(const auto& header : headers) {
        response += header + "\r\n";
    }
    response += "Content-Length: " + std::to_string(body.length()) + "\r\n";
    response += "\r\n";
    response += body;

    send(client_socket, response.c_str(), response.length(), 0);
}
