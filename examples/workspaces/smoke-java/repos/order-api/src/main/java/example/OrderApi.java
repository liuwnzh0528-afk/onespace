package example;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.CountDownLatch;

public final class OrderApi {
  private OrderApi() {}

  public static void main(String[] args) throws Exception {
    HttpServer server = HttpServer.create(new InetSocketAddress("0.0.0.0", 8080), 0);
    server.createContext("/health", exchange -> writeJSON(exchange, "{\"status\":\"ok\"}"));
    server.createContext("/", exchange -> writeJSON(exchange, "{\"service\":\"order-api\"}"));
    server.start();
    new CountDownLatch(1).await();
  }

  private static void writeJSON(HttpExchange exchange, String body) throws IOException {
    byte[] data = body.getBytes(StandardCharsets.UTF_8);
    exchange.getResponseHeaders().set("Content-Type", "application/json");
    exchange.sendResponseHeaders(200, data.length);
    try (OutputStream out = exchange.getResponseBody()) {
      out.write(data);
    }
  }
}
