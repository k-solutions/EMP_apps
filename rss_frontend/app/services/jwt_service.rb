class JwtService
  def self.generate_token(user_id:)
    payload = {
      sub: user_id,
      exp: (Time.current + 5.minutes).to_i
    }
    # Read ES256 EC Private Key (matching backend ec_public.pem pair)
    private_key_path = ENV.fetch("JWT_PRIVATE_KEY_PATH", Rails.root.join("keys", "ec_private.pem"))
    private_key_pem = File.read(private_key_path)
    private_key = OpenSSL::PKey::EC.new(private_key_pem)

    JWT.encode(payload, private_key, "ES256")
  end
end
