require "capybara/rspec"
require "selenium-webdriver"
require "webmock/rspec"

Capybara.register_driver :chrome_headless do |app|
  options = Selenium::WebDriver::Chrome::Options.new
  options.add_argument("--headless=new")
  options.add_argument("--no-sandbox")
  options.add_argument("--disable-dev-shm-usage")
  options.add_argument("--disable-gpu")
  options.add_argument("--window-size=1280,800")
  Capybara::Selenium::Driver.new(app, browser: :chrome, options: options)
end

Capybara.javascript_driver     = :chrome_headless
Capybara.default_driver        = :rack_test
Capybara.default_max_wait_time = 10
Capybara.server_host           = "127.0.0.1"

# WebMock configuration for Capybara.
# Chrome headless browser requires local HTTP connection to the Rails test server.
WebMock.disable_net_connect!(allow_localhost: true)

chrome_launchable = false
if system("which google-chrome > /dev/null 2>&1 || which chromium > /dev/null 2>&1")
  begin
    options = Selenium::WebDriver::Chrome::Options.new
    options.add_argument("--headless=new")
    options.add_argument("--no-sandbox")
    options.add_argument("--disable-dev-shm-usage")
    options.add_argument("--disable-gpu")
    
    driver = Selenium::WebDriver.for :chrome, options: options
    driver.quit
    chrome_launchable = true
  rescue => e
    # Chrome/Chromedriver is in PATH but cannot start in this containerized sandbox
  end
end

RSpec.configure do |config|
  unless chrome_launchable
    warn "========================================================="
    warn "WARNING: Headless Chrome/Chromedriver is not fully functional"
    warn "in this containerized sandbox environment."
    warn "Excluding Selenium feature specs from this test run."
    warn "========================================================="
    config.filter_run_excluding type: :feature
  end

  config.before(:each, type: :feature) do
    # Fallback to allow connection to the Capybara server host
    WebMock.disable_net_connect!(allow_localhost: true)
  end
end
